package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"eshop-builder/internal/database"
	"eshop-builder/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// GetOrders returns orders for a shop
func GetOrders(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset := (page - 1) * limit
	status := c.Query("status", "")
	search := c.Query("search", "")

	whereClause := "WHERE shop_id = $1"
	args := []interface{}{shopID}
	argIndex := 2

	if status != "" {
		whereClause += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}

	if search != "" {
		whereClause += fmt.Sprintf(" AND (order_number ILIKE $%d OR billing_email ILIKE $%d)", argIndex, argIndex)
		args = append(args, "%"+search+"%")
		argIndex++
	}

	// Count
	var total int64
	database.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM orders "+whereClause, args...,
	).Scan(&total)

	// Get orders
	args = append(args, limit, offset)
	rows, err := database.Pool.Query(context.Background(),
		fmt.Sprintf(`SELECT id, shop_id, customer_id, order_number, status, payment_status,
		        subtotal, shipping, tax, discount, total, currency,
		        shipping_first_name, shipping_last_name, billing_email,
		        payment_method, shipping_method, created_at, updated_at
		 FROM orders %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, whereClause, argIndex, argIndex+1),
		args...,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	defer rows.Close()

	orders := []models.Order{}
	for rows.Next() {
		var o models.Order
		rows.Scan(&o.ID, &o.ShopID, &o.CustomerID, &o.OrderNumber, &o.Status, &o.PaymentStatus,
			&o.Subtotal, &o.Shipping, &o.Tax, &o.Discount, &o.Total, &o.Currency,
			&o.ShippingFirstName, &o.ShippingLastName, &o.BillingEmail,
			&o.PaymentMethod, &o.ShippingMethod, &o.CreatedAt, &o.UpdatedAt)

		// Count items
		var itemCount int
		database.Pool.QueryRow(context.Background(),
			"SELECT COUNT(*) FROM order_items WHERE order_id = $1", o.ID,
		).Scan(&itemCount)

		orders = append(orders, o)
	}

	// Get status stats
	stats := make(map[string]int)
	statRows, _ := database.Pool.Query(context.Background(),
		"SELECT status, COUNT(*) FROM orders WHERE shop_id = $1 GROUP BY status", shopID,
	)
	for statRows.Next() {
		var s string
		var count int
		statRows.Scan(&s, &count)
		stats[s] = count
	}
	statRows.Close()

	return c.JSON(fiber.Map{
		"orders": orders,
		"total":  total,
		"page":   page,
		"limit":  limit,
		"stats":  stats,
	})
}

// GetOrder returns a single order
func GetOrder(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	orderID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid order ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	ctx := context.Background()

	var o models.Order
	err = database.Pool.QueryRow(ctx,
		`SELECT id, shop_id, customer_id, order_number, status, payment_status,
		        subtotal, shipping, tax, discount, total, currency,
		        shipping_first_name, shipping_last_name, shipping_company,
		        shipping_address, shipping_city, shipping_zip, shipping_country, shipping_phone,
		        billing_first_name, billing_last_name, billing_company,
		        billing_address, billing_city, billing_zip, billing_country, billing_phone, billing_email,
		        payment_method, payment_id, shipping_method, tracking_number,
		        customer_note, internal_note, coupon_code, coupon_id,
		        created_at, updated_at
		 FROM orders WHERE id = $1 AND shop_id = $2`,
		orderID, shopID,
	).Scan(&o.ID, &o.ShopID, &o.CustomerID, &o.OrderNumber, &o.Status, &o.PaymentStatus,
		&o.Subtotal, &o.Shipping, &o.Tax, &o.Discount, &o.Total, &o.Currency,
		&o.ShippingFirstName, &o.ShippingLastName, &o.ShippingCompany,
		&o.ShippingAddress, &o.ShippingCity, &o.ShippingZip, &o.ShippingCountry, &o.ShippingPhone,
		&o.BillingFirstName, &o.BillingLastName, &o.BillingCompany,
		&o.BillingAddress, &o.BillingCity, &o.BillingZip, &o.BillingCountry, &o.BillingPhone, &o.BillingEmail,
		&o.PaymentMethod, &o.PaymentID, &o.ShippingMethod, &o.TrackingNumber,
		&o.CustomerNote, &o.InternalNote, &o.CouponCode, &o.CouponID,
		&o.CreatedAt, &o.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	// Load items
	itemRows, _ := database.Pool.Query(ctx,
		`SELECT id, order_id, product_id, variant_id, name, sku, quantity, price, total,
		        variant_name, variant_options
		 FROM order_items WHERE order_id = $1`,
		o.ID,
	)
	for itemRows.Next() {
		var item models.OrderItem
		itemRows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.VariantID, &item.Name,
			&item.SKU, &item.Quantity, &item.Price, &item.Total, &item.VariantName, &item.VariantOptions)
		o.Items = append(o.Items, item)
	}
	itemRows.Close()

	// Load history
	histRows, _ := database.Pool.Query(ctx,
		"SELECT id, order_id, status, note, created_at FROM order_history WHERE order_id = $1 ORDER BY created_at DESC",
		o.ID,
	)
	for histRows.Next() {
		var h models.OrderHistory
		histRows.Scan(&h.ID, &h.OrderID, &h.Status, &h.Note, &h.CreatedAt)
		o.History = append(o.History, h)
	}
	histRows.Close()

	// Load customer if exists
	if o.CustomerID != nil {
		var customer models.Customer
		database.Pool.QueryRow(ctx,
			"SELECT id, email, first_name, last_name, phone FROM customers WHERE id = $1",
			*o.CustomerID,
		).Scan(&customer.ID, &customer.Email, &customer.FirstName, &customer.LastName, &customer.Phone)
		o.Customer = &customer
	}

	return c.JSON(o)
}

// UpdateOrder updates an order
func UpdateOrder(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	orderID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid order ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var updates struct {
		Status         *string `json:"status"`
		PaymentStatus  *string `json:"payment_status"`
		TrackingNumber *string `json:"tracking_number"`
		InternalNote   *string `json:"internal_note"`
	}

	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	ctx := context.Background()

	// Get current order
	var currentStatus string
	database.Pool.QueryRow(ctx,
		"SELECT status FROM orders WHERE id = $1 AND shop_id = $2",
		orderID, shopID,
	).Scan(&currentStatus)

	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIndex := 1

	if updates.Status != nil && *updates.Status != currentStatus {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *updates.Status)
		argIndex++

		// Add to history
		note := getStatusNote(*updates.Status)
		database.Pool.Exec(ctx,
			"INSERT INTO order_history (order_id, status, note) VALUES ($1, $2, $3)",
			orderID, *updates.Status, note,
		)
	}

	if updates.PaymentStatus != nil {
		setClauses = append(setClauses, fmt.Sprintf("payment_status = $%d", argIndex))
		args = append(args, *updates.PaymentStatus)
		argIndex++
	}

	if updates.TrackingNumber != nil {
		setClauses = append(setClauses, fmt.Sprintf("tracking_number = $%d", argIndex))
		args = append(args, *updates.TrackingNumber)
		argIndex++
	}

	if updates.InternalNote != nil {
		setClauses = append(setClauses, fmt.Sprintf("internal_note = $%d", argIndex))
		args = append(args, *updates.InternalNote)
		argIndex++
	}

	if len(args) > 0 {
		args = append(args, orderID, shopID)
		query := fmt.Sprintf("UPDATE orders SET %s WHERE id = $%d AND shop_id = $%d",
			strings.Join(setClauses, ", "), argIndex, argIndex+1)

		_, err = database.Pool.Exec(ctx, query, args...)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update order"})
		}
	}

	return c.JSON(fiber.Map{"success": true})
}

// CancelOrder cancels an order
func CancelOrder(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	orderID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid order ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	ctx := context.Background()

	// Check order status
	var status string
	err = database.Pool.QueryRow(ctx,
		"SELECT status FROM orders WHERE id = $1 AND shop_id = $2",
		orderID, shopID,
	).Scan(&status)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	if status != "pending" && status != "confirmed" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot cancel this order"})
	}

	// Restore inventory
	rows, _ := database.Pool.Query(ctx,
		"SELECT product_id, variant_id, quantity FROM order_items WHERE order_id = $1",
		orderID,
	)
	for rows.Next() {
		var productID, variantID *uuid.UUID
		var quantity int
		rows.Scan(&productID, &variantID, &quantity)

		if variantID != nil {
			database.Pool.Exec(ctx,
				"UPDATE product_variants SET quantity = quantity + $1 WHERE id = $2",
				quantity, *variantID,
			)
		} else if productID != nil {
			database.Pool.Exec(ctx,
				"UPDATE products SET quantity = quantity + $1 WHERE id = $2",
				quantity, *productID,
			)
		}
	}
	rows.Close()

	// Update order status
	database.Pool.Exec(ctx,
		"UPDATE orders SET status = 'cancelled', updated_at = NOW() WHERE id = $1",
		orderID,
	)

	database.Pool.Exec(ctx,
		"INSERT INTO order_history (order_id, status, note) VALUES ($1, 'cancelled', 'Objednávka zrušená')",
		orderID,
	)

	return c.JSON(fiber.Map{"success": true})
}

// CreatePublicOrder creates an order from storefront
func CreatePublicOrder(c *fiber.Ctx) error {
	shopSlug := c.Params("slug")

	var shopID uuid.UUID
	var shopCurrency string
	err := database.Pool.QueryRow(context.Background(),
		"SELECT id, currency FROM shops WHERE slug = $1 AND is_published = true",
		shopSlug,
	).Scan(&shopID, &shopCurrency)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req models.CreateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if len(req.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Order must have at least one item"})
	}

	ctx := context.Background()

	// Calculate totals
	var subtotal float64 = 0
	orderItems := []models.OrderItem{}

	for _, item := range req.Items {
		var product models.Product
		err := database.Pool.QueryRow(ctx,
			"SELECT id, name, price, sku, quantity, track_inventory FROM products WHERE id = $1 AND shop_id = $2 AND is_active = true",
			item.ProductID, shopID,
		).Scan(&product.ID, &product.Name, &product.Price, &product.SKU, &product.Quantity, &product.TrackInventory)

		if err != nil {
			continue
		}

		price := product.Price
		variantName := ""

		// Check variant
		if item.VariantID != nil {
			var variant models.ProductVariant
			database.Pool.QueryRow(ctx,
				"SELECT id, name, price, quantity FROM product_variants WHERE id = $1 AND product_id = $2",
				*item.VariantID, item.ProductID,
			).Scan(&variant.ID, &variant.Name, &variant.Price, &variant.Quantity)

			if variant.ID != uuid.Nil {
				price = variant.Price
				variantName = variant.Name
			}
		}

		itemTotal := price * float64(item.Quantity)
		subtotal += itemTotal

		orderItems = append(orderItems, models.OrderItem{
			ProductID:   &item.ProductID,
			VariantID:   item.VariantID,
			Name:        product.Name,
			SKU:         product.SKU,
			Quantity:    item.Quantity,
			Price:       price,
			Total:       itemTotal,
			VariantName: &variantName,
		})

		// Update inventory
		if product.TrackInventory {
			if item.VariantID != nil {
				database.Pool.Exec(ctx,
					"UPDATE product_variants SET quantity = quantity - $1 WHERE id = $2",
					item.Quantity, *item.VariantID,
				)
			} else {
				database.Pool.Exec(ctx,
					"UPDATE products SET quantity = quantity - $1 WHERE id = $2",
					item.Quantity, item.ProductID,
				)
			}
		}
	}

	// Get shipping cost
	var shippingCost float64 = 0
	if req.ShippingMethod != "" {
		database.Pool.QueryRow(ctx,
			"SELECT price, free_from FROM shipping_methods WHERE shop_id = $1 AND name = $2 AND is_active = true",
			shopID, req.ShippingMethod,
		).Scan(&shippingCost, nil)
	}

	// Apply coupon
	var discount float64 = 0
	var couponID *uuid.UUID
	if req.CouponCode != nil && *req.CouponCode != "" {
		var coupon models.Coupon
		err := database.Pool.QueryRow(ctx,
			`SELECT id, type, value, min_order_value FROM coupons 
			 WHERE shop_id = $1 AND code = $2 AND is_active = true
			 AND (expires_at IS NULL OR expires_at > NOW())`,
			shopID, strings.ToUpper(*req.CouponCode),
		).Scan(&coupon.ID, &coupon.Type, &coupon.Value, &coupon.MinOrderValue)

		if err == nil {
			if coupon.MinOrderValue == nil || *coupon.MinOrderValue <= subtotal {
				if coupon.Type == "percentage" {
					discount = subtotal * (coupon.Value / 100)
				} else {
					discount = coupon.Value
				}
				couponID = &coupon.ID

				// Update coupon usage
				database.Pool.Exec(ctx,
					"UPDATE coupons SET used_count = used_count + 1 WHERE id = $1",
					coupon.ID,
				)
			}
		}
	}

	// Tax (20% VAT)
	tax := (subtotal - discount + shippingCost) * 0.20
	total := subtotal - discount + shippingCost

	// Create/find customer
	var customerID *uuid.UUID
	if req.Billing.Email != "" {
		var existingCustomerID uuid.UUID
		err := database.Pool.QueryRow(ctx,
			"SELECT id FROM customers WHERE shop_id = $1 AND email = $2",
			shopID, strings.ToLower(req.Billing.Email),
		).Scan(&existingCustomerID)

		if err != nil {
			// Create new customer
			var newID uuid.UUID
			database.Pool.QueryRow(ctx,
				`INSERT INTO customers (shop_id, email, first_name, last_name, phone, address, city, zip, country)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
				shopID, strings.ToLower(req.Billing.Email), req.Billing.FirstName, req.Billing.LastName,
				req.Billing.Phone, req.Shipping.Address, req.Shipping.City, req.Shipping.Zip, req.Shipping.Country,
			).Scan(&newID)
			customerID = &newID
		} else {
			customerID = &existingCustomerID
		}
	}

	// Generate order number
	orderNumber := generateOrderNumber()

	// Create order
	var orderID uuid.UUID
	err = database.Pool.QueryRow(ctx,
		`INSERT INTO orders (shop_id, customer_id, order_number, status, payment_status,
		                     subtotal, shipping, tax, discount, total, currency,
		                     shipping_first_name, shipping_last_name, shipping_company,
		                     shipping_address, shipping_city, shipping_zip, shipping_country, shipping_phone,
		                     billing_first_name, billing_last_name, billing_company,
		                     billing_address, billing_city, billing_zip, billing_country, billing_phone, billing_email,
		                     payment_method, shipping_method, customer_note, coupon_code, coupon_id,
		                     created_at, updated_at)
		 VALUES ($1, $2, $3, 'pending', 'pending', $4, $5, $6, $7, $8, $9,
		         $10, $11, $12, $13, $14, $15, $16, $17,
		         $18, $19, $20, $21, $22, $23, $24, $25, $26,
		         $27, $28, $29, $30, $31, NOW(), NOW())
		 RETURNING id`,
		shopID, customerID, orderNumber, subtotal, shippingCost, tax, discount, total, shopCurrency,
		req.Shipping.FirstName, req.Shipping.LastName, req.Shipping.Company,
		req.Shipping.Address, req.Shipping.City, req.Shipping.Zip, req.Shipping.Country, req.Shipping.Phone,
		req.Billing.FirstName, req.Billing.LastName, req.Billing.Company,
		req.Billing.Address, req.Billing.City, req.Billing.Zip, req.Billing.Country, req.Billing.Phone, req.Billing.Email,
		req.PaymentMethod, req.ShippingMethod, req.CustomerNote, req.CouponCode, couponID,
	).Scan(&orderID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create order: " + err.Error()})
	}

	// Insert order items
	for _, item := range orderItems {
		variantOptionsJSON, _ := json.Marshal(item.VariantOptions)
		database.Pool.Exec(ctx,
			`INSERT INTO order_items (order_id, product_id, variant_id, name, sku, quantity, price, total, variant_name, variant_options)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			orderID, item.ProductID, item.VariantID, item.Name, item.SKU, item.Quantity, item.Price, item.Total, item.VariantName, variantOptionsJSON,
		)
	}

	// Add to history
	database.Pool.Exec(ctx,
		"INSERT INTO order_history (order_id, status, note) VALUES ($1, 'pending', 'Objednávka vytvorená')",
		orderID,
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":           orderID,
		"order_number": orderNumber,
		"total":        total,
		"currency":     shopCurrency,
	})
}

// GenerateInvoice creates an invoice for an order
func GenerateInvoice(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	orderID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid order ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	ctx := context.Background()

	// Get order
	var order models.Order
	err = database.Pool.QueryRow(ctx,
		`SELECT id, order_number, subtotal, tax, total, currency,
		        billing_first_name, billing_last_name, billing_company,
		        billing_address, billing_city, billing_zip, billing_country, billing_email,
		        payment_status
		 FROM orders WHERE id = $1 AND shop_id = $2`,
		orderID, shopID,
	).Scan(&order.ID, &order.OrderNumber, &order.Subtotal, &order.Tax, &order.Total, &order.Currency,
		&order.BillingFirstName, &order.BillingLastName, &order.BillingCompany,
		&order.BillingAddress, &order.BillingCity, &order.BillingZip, &order.BillingCountry, &order.BillingEmail,
		&order.PaymentStatus)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	// Get shop settings
	var settings models.ShopSettings
	database.Pool.QueryRow(ctx,
		`SELECT company_name, ico, dic, ic_dph, iban, invoice_prefix, invoice_next_number
		 FROM shop_settings WHERE shop_id = $1`,
		shopID,
	).Scan(&settings.CompanyName, &settings.ICO, &settings.DIC, &settings.ICDPH,
		&settings.IBAN, &settings.InvoicePrefix, &settings.InvoiceNextNumber)

	// Get order items
	itemRows, _ := database.Pool.Query(ctx,
		"SELECT name, quantity, price, total FROM order_items WHERE order_id = $1",
		orderID,
	)

	items := []map[string]interface{}{}
	for itemRows.Next() {
		var name string
		var quantity int
		var price, total float64
		itemRows.Scan(&name, &quantity, &price, &total)
		items = append(items, map[string]interface{}{
			"name":     name,
			"quantity": quantity,
			"price":    price,
			"total":    total,
		})
	}
	itemRows.Close()

	itemsJSON, _ := json.Marshal(items)

	// Generate invoice number
	invoiceNumber := fmt.Sprintf("%s%d%06d", settings.InvoicePrefix, time.Now().Year(), settings.InvoiceNextNumber)

	// Update next number
	database.Pool.Exec(ctx,
		"UPDATE shop_settings SET invoice_next_number = invoice_next_number + 1 WHERE shop_id = $1",
		shopID,
	)

	dueDate := time.Now().AddDate(0, 0, 14) // 14 days
	status := "unpaid"
	var paidAt *time.Time
	if order.PaymentStatus == "paid" || order.PaymentStatus == "completed" {
		status = "paid"
		now := time.Now()
		paidAt = &now
	}

	customerName := ""
	if order.BillingFirstName != nil {
		customerName = *order.BillingFirstName
	}
	if order.BillingLastName != nil {
		customerName += " " + *order.BillingLastName
	}

	var invoiceID uuid.UUID
	err = database.Pool.QueryRow(ctx,
		`INSERT INTO invoices (shop_id, invoice_number, type, issue_date, due_date, paid_at, status,
		                       subtotal, tax, total, currency,
		                       supplier_name, supplier_ico, supplier_dic, supplier_ic_dph, supplier_iban,
		                       customer_name, customer_address, customer_city, customer_zip, customer_country, customer_email,
		                       items, order_id, order_number)
		 VALUES ($1, $2, 'invoice', NOW(), $3, $4, $5, $6, $7, $8, $9,
		         $10, $11, $12, $13, $14,
		         $15, $16, $17, $18, $19, $20,
		         $21, $22, $23)
		 RETURNING id`,
		shopID, invoiceNumber, dueDate, paidAt, status, order.Subtotal, order.Tax, order.Total, order.Currency,
		settings.CompanyName, settings.ICO, settings.DIC, settings.ICDPH, settings.IBAN,
		customerName, order.BillingAddress, order.BillingCity, order.BillingZip, order.BillingCountry, order.BillingEmail,
		itemsJSON, orderID, order.OrderNumber,
	).Scan(&invoiceID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create invoice: " + err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":             invoiceID,
		"invoice_number": invoiceNumber,
	})
}

func getStatusNote(status string) string {
	notes := map[string]string{
		"pending":    "Objednávka čaká na spracovanie",
		"confirmed":  "Objednávka potvrdená",
		"processing": "Objednávka sa pripravuje",
		"shipped":    "Objednávka odoslaná",
		"delivered":  "Objednávka doručená",
		"cancelled":  "Objednávka zrušená",
		"refunded":   "Objednávka vrátená",
	}
	if note, ok := notes[status]; ok {
		return note
	}
	return "Stav zmenený na: " + status
}
