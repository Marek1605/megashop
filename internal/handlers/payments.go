package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"eshop-builder/internal/database"
	"eshop-builder/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ========================================
// GOPAY
// ========================================

func CreateGoPayPayment(c *fiber.Ctx) error {
	var req struct {
		OrderID   string `json:"order_id"`
		ReturnURL string `json:"return_url"`
	}
	c.BodyParser(&req)

	orderID, _ := uuid.Parse(req.OrderID)

	var order models.Order
	err := database.Pool.QueryRow(context.Background(),
		"SELECT id, order_number, total, currency, billing_email FROM orders WHERE id = $1",
		orderID,
	).Scan(&order.ID, &order.OrderNumber, &order.Total, &order.Currency, &order.BillingEmail)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	// Get GoPay access token
	clientID := os.Getenv("GOPAY_CLIENT_ID")
	clientSecret := os.Getenv("GOPAY_CLIENT_SECRET")
	goID := os.Getenv("GOPAY_GOID")
	apiURL := os.Getenv("GOPAY_API_URL")
	if apiURL == "" {
		apiURL = "https://gate.gopay.cz/api"
	}

	// OAuth token
	tokenReq, _ := http.NewRequest("POST", apiURL+"/oauth2/token",
		strings.NewReader("grant_type=client_credentials&scope=payment-create"))
	tokenReq.SetBasicAuth(clientID, clientSecret)
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tokenResp, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "GoPay auth failed"})
	}
	defer tokenResp.Body.Close()

	var tokenData struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(tokenResp.Body).Decode(&tokenData)

	// Create payment
	email := ""
	if order.BillingEmail != nil {
		email = *order.BillingEmail
	}

	paymentBody := map[string]interface{}{
		"payer": map[string]interface{}{
			"default_payment_instrument": "PAYMENT_CARD",
			"contact": map[string]string{
				"email": email,
			},
		},
		"amount":   int(order.Total * 100),
		"currency": order.Currency,
		"order_number": order.OrderNumber,
		"order_description": "Objednávka " + order.OrderNumber,
		"callback": map[string]string{
			"return_url":       req.ReturnURL,
			"notification_url": os.Getenv("NEXTAUTH_URL") + "/api/v1/webhooks/gopay",
		},
		"lang": "SK",
		"target": map[string]interface{}{
			"type":  "ACCOUNT",
			"goid":  goID,
		},
	}

	bodyJSON, _ := json.Marshal(paymentBody)
	payReq, _ := http.NewRequest("POST", apiURL+"/payments/payment", bytes.NewReader(bodyJSON))
	payReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	payReq.Header.Set("Content-Type", "application/json")

	payResp, err := http.DefaultClient.Do(payReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "GoPay payment failed"})
	}
	defer payResp.Body.Close()

	var payData struct {
		ID      int64  `json:"id"`
		GwURL   string `json:"gw_url"`
		State   string `json:"state"`
	}
	json.NewDecoder(payResp.Body).Decode(&payData)

	// Save payment
	database.Pool.Exec(context.Background(),
		`INSERT INTO payments (gateway, gateway_id, order_id, order_number, amount, currency, status)
		 VALUES ('gopay', $1, $2, $3, $4, $5, 'pending')`,
		fmt.Sprintf("%d", payData.ID), orderID, order.OrderNumber, order.Total, order.Currency)

	return c.JSON(fiber.Map{
		"payment_id":   payData.ID,
		"redirect_url": payData.GwURL,
	})
}

func GoPayWebhook(c *fiber.Ctx) error {
	var body struct {
		ID    int64  `json:"id"`
		State string `json:"state"`
	}
	c.BodyParser(&body)

	gatewayID := fmt.Sprintf("%d", body.ID)

	statusMap := map[string]string{
		"CREATED":   "pending",
		"PAID":      "completed",
		"CANCELED":  "failed",
		"TIMEOUTED": "failed",
		"REFUNDED":  "refunded",
	}

	status := statusMap[body.State]
	if status == "" {
		status = "pending"
	}

	ctx := context.Background()

	// Update payment
	var orderID uuid.UUID
	database.Pool.QueryRow(ctx,
		"UPDATE payments SET status = $1, updated_at = NOW() WHERE gateway_id = $2 RETURNING order_id",
		status, gatewayID).Scan(&orderID)

	// Update order
	database.Pool.Exec(ctx,
		"UPDATE orders SET payment_status = $1, updated_at = NOW() WHERE id = $2",
		status, orderID)

	if status == "completed" {
		database.Pool.Exec(ctx,
			"UPDATE orders SET status = 'confirmed' WHERE id = $1", orderID)
		database.Pool.Exec(ctx,
			"INSERT INTO order_history (order_id, status, note) VALUES ($1, 'confirmed', 'Platba prijatá cez GoPay')",
			orderID)
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// ========================================
// STRIPE
// ========================================

func CreateStripePayment(c *fiber.Ctx) error {
	var req struct {
		OrderID    string `json:"order_id"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	c.BodyParser(&req)

	orderID, _ := uuid.Parse(req.OrderID)

	var order models.Order
	err := database.Pool.QueryRow(context.Background(),
		`SELECT id, order_number, total, currency, billing_email FROM orders WHERE id = $1`,
		orderID,
	).Scan(&order.ID, &order.OrderNumber, &order.Total, &order.Currency, &order.BillingEmail)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	// Get order items
	rows, _ := database.Pool.Query(context.Background(),
		"SELECT name, quantity, price FROM order_items WHERE order_id = $1", orderID)
	defer rows.Close()

	lineItems := []map[string]interface{}{}
	for rows.Next() {
		var name string
		var quantity int
		var price float64
		rows.Scan(&name, &quantity, &price)
		lineItems = append(lineItems, map[string]interface{}{
			"price_data": map[string]interface{}{
				"currency": strings.ToLower(order.Currency),
				"product_data": map[string]string{
					"name": name,
				},
				"unit_amount": int(price * 100),
			},
			"quantity": quantity,
		})
	}

	// Create Stripe checkout session
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")

	data := url.Values{}
	data.Set("mode", "payment")
	data.Set("success_url", req.SuccessURL+"?session_id={CHECKOUT_SESSION_ID}")
	data.Set("cancel_url", req.CancelURL)

	for i, item := range lineItems {
		priceData := item["price_data"].(map[string]interface{})
		productData := priceData["product_data"].(map[string]string)
		data.Set(fmt.Sprintf("line_items[%d][price_data][currency]", i), priceData["currency"].(string))
		data.Set(fmt.Sprintf("line_items[%d][price_data][product_data][name]", i), productData["name"])
		data.Set(fmt.Sprintf("line_items[%d][price_data][unit_amount]", i), fmt.Sprintf("%d", priceData["unit_amount"].(int)))
		data.Set(fmt.Sprintf("line_items[%d][quantity]", i), fmt.Sprintf("%d", item["quantity"].(int)))
	}

	stripeReq, _ := http.NewRequest("POST", "https://api.stripe.com/v1/checkout/sessions",
		strings.NewReader(data.Encode()))
	stripeReq.SetBasicAuth(stripeKey, "")
	stripeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	stripeResp, err := http.DefaultClient.Do(stripeReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Stripe failed"})
	}
	defer stripeResp.Body.Close()

	var sessionData struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	json.NewDecoder(stripeResp.Body).Decode(&sessionData)

	// Save payment
	database.Pool.Exec(context.Background(),
		`INSERT INTO payments (gateway, gateway_id, order_id, order_number, amount, currency, status)
		 VALUES ('stripe', $1, $2, $3, $4, $5, 'pending')`,
		sessionData.ID, orderID, order.OrderNumber, order.Total, order.Currency)

	return c.JSON(fiber.Map{
		"session_id":   sessionData.ID,
		"redirect_url": sessionData.URL,
	})
}

func StripeWebhook(c *fiber.Ctx) error {
	body := c.Body()

	var event struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID string `json:"id"`
			} `json:"object"`
		} `json:"data"`
	}
	json.Unmarshal(body, &event)

	ctx := context.Background()

	if event.Type == "checkout.session.completed" {
		sessionID := event.Data.Object.ID

		var orderID uuid.UUID
		database.Pool.QueryRow(ctx,
			"UPDATE payments SET status = 'completed', updated_at = NOW() WHERE gateway_id = $1 RETURNING order_id",
			sessionID).Scan(&orderID)

		database.Pool.Exec(ctx,
			"UPDATE orders SET payment_status = 'completed', status = 'confirmed' WHERE id = $1", orderID)

		database.Pool.Exec(ctx,
			"INSERT INTO order_history (order_id, status, note) VALUES ($1, 'confirmed', 'Platba prijatá cez Stripe')",
			orderID)
	}

	return c.JSON(fiber.Map{"received": true})
}

// ========================================
// COMGATE
// ========================================

func CreateComGatePayment(c *fiber.Ctx) error {
	var req struct {
		OrderID   string `json:"order_id"`
		ReturnURL string `json:"return_url"`
	}
	c.BodyParser(&req)

	orderID, _ := uuid.Parse(req.OrderID)

	var order models.Order
	err := database.Pool.QueryRow(context.Background(),
		"SELECT id, order_number, total, currency, billing_email, billing_country FROM orders WHERE id = $1",
		orderID,
	).Scan(&order.ID, &order.OrderNumber, &order.Total, &order.Currency, &order.BillingEmail, &order.BillingCountry)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	merchant := os.Getenv("COMGATE_MERCHANT")
	secret := os.Getenv("COMGATE_SECRET")
	isTest := os.Getenv("COMGATE_TEST") == "true"

	email := ""
	if order.BillingEmail != nil {
		email = *order.BillingEmail
	}
	country := "SK"
	if order.BillingCountry != nil {
		country = *order.BillingCountry
	}

	data := url.Values{}
	data.Set("merchant", merchant)
	data.Set("secret", secret)
	data.Set("test", fmt.Sprintf("%t", isTest))
	data.Set("price", fmt.Sprintf("%d", int(order.Total*100)))
	data.Set("curr", order.Currency)
	data.Set("label", "Objednávka "+order.OrderNumber)
	data.Set("refId", order.OrderNumber)
	data.Set("email", email)
	data.Set("prepareOnly", "true")
	data.Set("country", country)
	data.Set("lang", "sk")
	data.Set("method", "ALL")
	data.Set("url", req.ReturnURL)
	data.Set("urlc", os.Getenv("NEXTAUTH_URL")+"/api/v1/webhooks/comgate")

	resp, err := http.PostForm("https://payments.comgate.cz/v1.0/create", data)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "ComGate failed"})
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	result := parseComGateResponse(string(bodyBytes))

	if result["code"] != "0" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result["message"]})
	}

	// Save payment
	database.Pool.Exec(context.Background(),
		`INSERT INTO payments (gateway, gateway_id, order_id, order_number, amount, currency, status)
		 VALUES ('comgate', $1, $2, $3, $4, $5, 'pending')`,
		result["transId"], orderID, order.OrderNumber, order.Total, order.Currency)

	return c.JSON(fiber.Map{
		"payment_id":   result["transId"],
		"redirect_url": result["redirect"],
	})
}

func ComGateWebhook(c *fiber.Ctx) error {
	transId := c.FormValue("transId")
	status := c.FormValue("status")

	statusMap := map[string]string{
		"PENDING":    "pending",
		"PAID":       "completed",
		"AUTHORIZED": "completed",
		"CANCELLED":  "failed",
		"TIMEOUTED":  "failed",
		"REFUNDED":   "refunded",
	}

	mappedStatus := statusMap[status]
	if mappedStatus == "" {
		mappedStatus = "pending"
	}

	ctx := context.Background()

	var orderID uuid.UUID
	database.Pool.QueryRow(ctx,
		"UPDATE payments SET status = $1, updated_at = NOW() WHERE gateway_id = $2 RETURNING order_id",
		mappedStatus, transId).Scan(&orderID)

	database.Pool.Exec(ctx,
		"UPDATE orders SET payment_status = $1, updated_at = NOW() WHERE id = $2",
		mappedStatus, orderID)

	if mappedStatus == "completed" {
		database.Pool.Exec(ctx,
			"UPDATE orders SET status = 'confirmed' WHERE id = $1", orderID)
		database.Pool.Exec(ctx,
			"INSERT INTO order_history (order_id, status, note) VALUES ($1, 'confirmed', 'Platba prijatá cez ComGate')",
			orderID)
	}

	return c.SendString("code=0&message=OK")
}

func GetPaymentStatus(c *fiber.Ctx) error {
	paymentID, _ := uuid.Parse(c.Params("id"))

	var payment models.Payment
	err := database.Pool.QueryRow(context.Background(),
		"SELECT id, gateway, gateway_id, order_id, amount, currency, status FROM payments WHERE id = $1",
		paymentID,
	).Scan(&payment.ID, &payment.Gateway, &payment.GatewayID, &payment.OrderID,
		&payment.Amount, &payment.Currency, &payment.Status)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payment not found"})
	}

	return c.JSON(payment)
}

func parseComGateResponse(text string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(text, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[kv[0]], _ = url.QueryUnescape(kv[1])
		}
	}
	return result
}
