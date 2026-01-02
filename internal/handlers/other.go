package handlers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"eshop-builder/internal/database"
	"eshop-builder/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ========================================
// CATEGORIES
// ========================================

func GetCategories(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	rows, _ := database.Pool.Query(context.Background(),
		`SELECT id, shop_id, parent_id, name, slug, description, image, position, created_at, updated_at
		 FROM categories WHERE shop_id = $1 ORDER BY position`, shopID)
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var cat models.Category
		rows.Scan(&cat.ID, &cat.ShopID, &cat.ParentID, &cat.Name, &cat.Slug, &cat.Description,
			&cat.Image, &cat.Position, &cat.CreatedAt, &cat.UpdatedAt)

		// Count products
		database.Pool.QueryRow(context.Background(),
			"SELECT COUNT(*) FROM products WHERE category_id = $1", cat.ID).Scan(&cat.ProductCount)

		categories = append(categories, cat)
	}

	return c.JSON(fiber.Map{"categories": categories})
}

func CreateCategory(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req struct {
		Name        string     `json:"name"`
		Description *string    `json:"description"`
		Image       *string    `json:"image"`
		ParentID    *uuid.UUID `json:"parent_id"`
	}
	c.BodyParser(&req)

	slug := generateSlug(req.Name)

	var cat models.Category
	database.Pool.QueryRow(context.Background(),
		`INSERT INTO categories (shop_id, parent_id, name, slug, description, image, position)
		 VALUES ($1, $2, $3, $4, $5, $6, (SELECT COALESCE(MAX(position), 0) + 1 FROM categories WHERE shop_id = $1))
		 RETURNING id, shop_id, parent_id, name, slug, description, image, position, created_at, updated_at`,
		shopID, req.ParentID, req.Name, slug, req.Description, req.Image,
	).Scan(&cat.ID, &cat.ShopID, &cat.ParentID, &cat.Name, &cat.Slug, &cat.Description,
		&cat.Image, &cat.Position, &cat.CreatedAt, &cat.UpdatedAt)

	return c.Status(fiber.StatusCreated).JSON(cat)
}

func UpdateCategory(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	catID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var updates map[string]interface{}
	c.BodyParser(&updates)

	// Simple update for categories
	if name, ok := updates["name"].(string); ok {
		database.Pool.Exec(context.Background(),
			"UPDATE categories SET name = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3",
			name, catID, shopID)
	}

	return c.JSON(fiber.Map{"success": true})
}

func DeleteCategory(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	catID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	database.Pool.Exec(context.Background(),
		"DELETE FROM categories WHERE id = $1 AND shop_id = $2", catID, shopID)

	return c.JSON(fiber.Map{"success": true})
}

func GetPublicCategories(c *fiber.Ctx) error {
	slug := c.Params("slug")

	var shopID uuid.UUID
	database.Pool.QueryRow(context.Background(),
		"SELECT id FROM shops WHERE slug = $1 AND is_published = true", slug).Scan(&shopID)

	rows, _ := database.Pool.Query(context.Background(),
		"SELECT id, name, slug, description, image FROM categories WHERE shop_id = $1 ORDER BY position", shopID)
	defer rows.Close()

	categories := []map[string]interface{}{}
	for rows.Next() {
		var id uuid.UUID
		var name, catSlug string
		var desc, img *string
		rows.Scan(&id, &name, &catSlug, &desc, &img)
		categories = append(categories, map[string]interface{}{
			"id": id, "name": name, "slug": catSlug, "description": desc, "image": img,
		})
	}

	return c.JSON(fiber.Map{"categories": categories})
}

// ========================================
// CUSTOMERS
// ========================================

func GetCustomers(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset := (page - 1) * limit
	search := c.Query("search", "")

	whereClause := "WHERE shop_id = $1"
	args := []interface{}{shopID}
	argIndex := 2

	if search != "" {
		whereClause += fmt.Sprintf(" AND (email ILIKE $%d OR first_name ILIKE $%d OR last_name ILIKE $%d)", argIndex, argIndex, argIndex)
		args = append(args, "%"+search+"%")
		argIndex++
	}

	var total int64
	database.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM customers "+whereClause, args...).Scan(&total)

	args = append(args, limit, offset)
	rows, _ := database.Pool.Query(context.Background(),
		fmt.Sprintf(`SELECT id, shop_id, email, first_name, last_name, phone, city, country, created_at
		 FROM customers %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, whereClause, argIndex, argIndex+1), args...)
	defer rows.Close()

	customers := []models.Customer{}
	for rows.Next() {
		var cust models.Customer
		rows.Scan(&cust.ID, &cust.ShopID, &cust.Email, &cust.FirstName, &cust.LastName,
			&cust.Phone, &cust.City, &cust.Country, &cust.CreatedAt)

		database.Pool.QueryRow(context.Background(),
			"SELECT COUNT(*), COALESCE(SUM(total), 0) FROM orders WHERE customer_id = $1 AND payment_status IN ('paid', 'completed')",
			cust.ID).Scan(&cust.OrdersCount, &cust.TotalSpent)

		customers = append(customers, cust)
	}

	return c.JSON(fiber.Map{"customers": customers, "total": total, "page": page, "limit": limit})
}

func GetCustomer(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	custID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var cust models.Customer
	err = database.Pool.QueryRow(context.Background(),
		`SELECT id, shop_id, email, first_name, last_name, phone, address, city, zip, country,
		        accepts_marketing, notes, created_at, updated_at
		 FROM customers WHERE id = $1 AND shop_id = $2`, custID, shopID,
	).Scan(&cust.ID, &cust.ShopID, &cust.Email, &cust.FirstName, &cust.LastName, &cust.Phone,
		&cust.Address, &cust.City, &cust.Zip, &cust.Country, &cust.AcceptsMarketing, &cust.Notes,
		&cust.CreatedAt, &cust.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Customer not found"})
	}

	return c.JSON(cust)
}

func UpdateCustomer(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	custID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var updates map[string]interface{}
	c.BodyParser(&updates)

	if notes, ok := updates["notes"].(string); ok {
		database.Pool.Exec(context.Background(),
			"UPDATE customers SET notes = $1, updated_at = NOW() WHERE id = $2 AND shop_id = $3",
			notes, custID, shopID)
	}

	return c.JSON(fiber.Map{"success": true})
}

// ========================================
// SHIPPING METHODS
// ========================================

func GetShippingMethods(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	rows, _ := database.Pool.Query(context.Background(),
		`SELECT id, shop_id, name, description, price, free_from, estimated_days, carrier, is_active, position
		 FROM shipping_methods WHERE shop_id = $1 ORDER BY position`, shopID)
	defer rows.Close()

	methods := []models.ShippingMethod{}
	for rows.Next() {
		var m models.ShippingMethod
		rows.Scan(&m.ID, &m.ShopID, &m.Name, &m.Description, &m.Price, &m.FreeFrom,
			&m.EstimatedDays, &m.Carrier, &m.IsActive, &m.Position)
		methods = append(methods, m)
	}

	return c.JSON(fiber.Map{"methods": methods})
}

func CreateShippingMethod(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req models.ShippingMethod
	c.BodyParser(&req)

	var m models.ShippingMethod
	database.Pool.QueryRow(context.Background(),
		`INSERT INTO shipping_methods (shop_id, name, description, price, free_from, estimated_days, carrier, is_active, position)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (SELECT COALESCE(MAX(position), 0) + 1 FROM shipping_methods WHERE shop_id = $1))
		 RETURNING id, shop_id, name, description, price, free_from, estimated_days, carrier, is_active, position`,
		shopID, req.Name, req.Description, req.Price, req.FreeFrom, req.EstimatedDays, req.Carrier, true,
	).Scan(&m.ID, &m.ShopID, &m.Name, &m.Description, &m.Price, &m.FreeFrom, &m.EstimatedDays, &m.Carrier, &m.IsActive, &m.Position)

	return c.Status(fiber.StatusCreated).JSON(m)
}

func UpdateShippingMethod(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	methodID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req models.ShippingMethod
	c.BodyParser(&req)

	database.Pool.Exec(context.Background(),
		`UPDATE shipping_methods SET name = $1, description = $2, price = $3, free_from = $4,
		 estimated_days = $5, carrier = $6, is_active = $7, updated_at = NOW()
		 WHERE id = $8 AND shop_id = $9`,
		req.Name, req.Description, req.Price, req.FreeFrom, req.EstimatedDays, req.Carrier, req.IsActive,
		methodID, shopID)

	return c.JSON(fiber.Map{"success": true})
}

func DeleteShippingMethod(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	methodID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	database.Pool.Exec(context.Background(),
		"DELETE FROM shipping_methods WHERE id = $1 AND shop_id = $2", methodID, shopID)

	return c.JSON(fiber.Map{"success": true})
}

func GetPublicShippingMethods(c *fiber.Ctx) error {
	slug := c.Params("slug")

	var shopID uuid.UUID
	database.Pool.QueryRow(context.Background(),
		"SELECT id FROM shops WHERE slug = $1 AND is_published = true", slug).Scan(&shopID)

	rows, _ := database.Pool.Query(context.Background(),
		"SELECT id, name, description, price, free_from, estimated_days FROM shipping_methods WHERE shop_id = $1 AND is_active = true ORDER BY position",
		shopID)
	defer rows.Close()

	methods := []map[string]interface{}{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		var desc *string
		var price float64
		var freeFrom *float64
		var days *string
		rows.Scan(&id, &name, &desc, &price, &freeFrom, &days)
		methods = append(methods, map[string]interface{}{
			"id": id, "name": name, "description": desc, "price": price, "free_from": freeFrom, "estimated_days": days,
		})
	}

	return c.JSON(fiber.Map{"methods": methods})
}

// ========================================
// PAYMENT METHODS
// ========================================

func GetPaymentMethods(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	rows, _ := database.Pool.Query(context.Background(),
		"SELECT id, shop_id, name, description, type, fee, instructions, is_active, position FROM payment_methods WHERE shop_id = $1 ORDER BY position",
		shopID)
	defer rows.Close()

	methods := []models.PaymentMethod{}
	for rows.Next() {
		var m models.PaymentMethod
		rows.Scan(&m.ID, &m.ShopID, &m.Name, &m.Description, &m.Type, &m.Fee, &m.Instructions, &m.IsActive, &m.Position)
		methods = append(methods, m)
	}

	return c.JSON(fiber.Map{"methods": methods})
}

func CreatePaymentMethod(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req models.PaymentMethod
	c.BodyParser(&req)

	var m models.PaymentMethod
	database.Pool.QueryRow(context.Background(),
		`INSERT INTO payment_methods (shop_id, name, description, type, fee, instructions, is_active, position)
		 VALUES ($1, $2, $3, $4, $5, $6, true, (SELECT COALESCE(MAX(position), 0) + 1 FROM payment_methods WHERE shop_id = $1))
		 RETURNING id, shop_id, name, description, type, fee, instructions, is_active, position`,
		shopID, req.Name, req.Description, req.Type, req.Fee, req.Instructions,
	).Scan(&m.ID, &m.ShopID, &m.Name, &m.Description, &m.Type, &m.Fee, &m.Instructions, &m.IsActive, &m.Position)

	return c.Status(fiber.StatusCreated).JSON(m)
}

func UpdatePaymentMethod(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	methodID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req models.PaymentMethod
	c.BodyParser(&req)

	database.Pool.Exec(context.Background(),
		`UPDATE payment_methods SET name = $1, description = $2, type = $3, fee = $4, instructions = $5, is_active = $6, updated_at = NOW()
		 WHERE id = $7 AND shop_id = $8`,
		req.Name, req.Description, req.Type, req.Fee, req.Instructions, req.IsActive, methodID, shopID)

	return c.JSON(fiber.Map{"success": true})
}

func DeletePaymentMethod(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	methodID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	database.Pool.Exec(context.Background(),
		"DELETE FROM payment_methods WHERE id = $1 AND shop_id = $2", methodID, shopID)

	return c.JSON(fiber.Map{"success": true})
}

func GetPublicPaymentMethods(c *fiber.Ctx) error {
	slug := c.Params("slug")

	var shopID uuid.UUID
	database.Pool.QueryRow(context.Background(),
		"SELECT id FROM shops WHERE slug = $1 AND is_published = true", slug).Scan(&shopID)

	rows, _ := database.Pool.Query(context.Background(),
		"SELECT id, name, description, type, fee FROM payment_methods WHERE shop_id = $1 AND is_active = true ORDER BY position",
		shopID)
	defer rows.Close()

	methods := []map[string]interface{}{}
	for rows.Next() {
		var id uuid.UUID
		var name, typ string
		var desc *string
		var fee float64
		rows.Scan(&id, &name, &desc, &typ, &fee)
		methods = append(methods, map[string]interface{}{
			"id": id, "name": name, "description": desc, "type": typ, "fee": fee,
		})
	}

	return c.JSON(fiber.Map{"methods": methods})
}

// ========================================
// COUPONS
// ========================================

func GetCoupons(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	rows, _ := database.Pool.Query(context.Background(),
		`SELECT id, shop_id, code, description, type, value, min_order_value, max_uses, used_count,
		        starts_at, expires_at, is_active, created_at
		 FROM coupons WHERE shop_id = $1 ORDER BY created_at DESC`, shopID)
	defer rows.Close()

	coupons := []models.Coupon{}
	for rows.Next() {
		var coupon models.Coupon
		rows.Scan(&coupon.ID, &coupon.ShopID, &coupon.Code, &coupon.Description, &coupon.Type,
			&coupon.Value, &coupon.MinOrderValue, &coupon.MaxUses, &coupon.UsedCount,
			&coupon.StartsAt, &coupon.ExpiresAt, &coupon.IsActive, &coupon.CreatedAt)
		coupons = append(coupons, coupon)
	}

	return c.JSON(fiber.Map{"coupons": coupons})
}

func CreateCoupon(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req models.Coupon
	c.BodyParser(&req)

	if req.Type == "" {
		req.Type = "percentage"
	}

	var coupon models.Coupon
	database.Pool.QueryRow(context.Background(),
		`INSERT INTO coupons (shop_id, code, description, type, value, min_order_value, max_uses, starts_at, expires_at, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true)
		 RETURNING id, shop_id, code, description, type, value, min_order_value, max_uses, used_count, starts_at, expires_at, is_active, created_at`,
		shopID, req.Code, req.Description, req.Type, req.Value, req.MinOrderValue, req.MaxUses, req.StartsAt, req.ExpiresAt,
	).Scan(&coupon.ID, &coupon.ShopID, &coupon.Code, &coupon.Description, &coupon.Type,
		&coupon.Value, &coupon.MinOrderValue, &coupon.MaxUses, &coupon.UsedCount,
		&coupon.StartsAt, &coupon.ExpiresAt, &coupon.IsActive, &coupon.CreatedAt)

	return c.Status(fiber.StatusCreated).JSON(coupon)
}

func UpdateCoupon(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	couponID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req models.Coupon
	c.BodyParser(&req)

	database.Pool.Exec(context.Background(),
		`UPDATE coupons SET code = $1, description = $2, type = $3, value = $4,
		 min_order_value = $5, max_uses = $6, expires_at = $7, is_active = $8, updated_at = NOW()
		 WHERE id = $9 AND shop_id = $10`,
		req.Code, req.Description, req.Type, req.Value, req.MinOrderValue, req.MaxUses,
		req.ExpiresAt, req.IsActive, couponID, shopID)

	return c.JSON(fiber.Map{"success": true})
}

func DeleteCoupon(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	couponID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	database.Pool.Exec(context.Background(),
		"DELETE FROM coupons WHERE id = $1 AND shop_id = $2", couponID, shopID)

	return c.JSON(fiber.Map{"success": true})
}

// ========================================
// ANALYTICS
// ========================================

func GetAnalytics(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	period, _ := strconv.Atoi(c.Query("period", "30"))
	startDate := time.Now().AddDate(0, 0, -period)

	ctx := context.Background()

	var summary models.AnalyticsSummary

	// Orders stats
	database.Pool.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(total), 0)
		 FROM orders WHERE shop_id = $1 AND payment_status IN ('paid', 'completed') AND created_at >= $2`,
		shopID, startDate).Scan(&summary.TotalOrders, &summary.TotalRevenue)

	database.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM orders WHERE shop_id = $1 AND status = 'delivered' AND created_at >= $2",
		shopID, startDate).Scan(&summary.CompletedOrders)

	database.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM orders WHERE shop_id = $1 AND status = 'pending' AND created_at >= $2",
		shopID, startDate).Scan(&summary.PendingOrders)

	database.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM orders WHERE shop_id = $1 AND status = 'cancelled' AND created_at >= $2",
		shopID, startDate).Scan(&summary.CancelledOrders)

	if summary.TotalOrders > 0 {
		summary.AverageOrderValue = summary.TotalRevenue / float64(summary.TotalOrders)
	}

	// Customers
	database.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM customers WHERE shop_id = $1", shopID).Scan(&summary.TotalCustomers)

	database.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM customers WHERE shop_id = $1 AND created_at >= $2",
		shopID, startDate).Scan(&summary.NewCustomers)

	// Daily stats
	dailyRows, _ := database.Pool.Query(ctx,
		`SELECT DATE(created_at) as date, COUNT(*), COALESCE(SUM(total), 0)
		 FROM orders WHERE shop_id = $1 AND created_at >= $2
		 GROUP BY DATE(created_at) ORDER BY date`,
		shopID, startDate)
	defer dailyRows.Close()

	dailyStats := []models.DailyStats{}
	for dailyRows.Next() {
		var ds models.DailyStats
		var date time.Time
		dailyRows.Scan(&date, &ds.Orders, &ds.Revenue)
		ds.Date = date
		ds.ShopID = shopID
		dailyStats = append(dailyStats, ds)
	}

	// Top products
	topRows, _ := database.Pool.Query(ctx,
		`SELECT oi.product_id, oi.name, SUM(oi.quantity), SUM(oi.total)
		 FROM order_items oi
		 JOIN orders o ON oi.order_id = o.id
		 WHERE o.shop_id = $1 AND o.created_at >= $2 AND oi.product_id IS NOT NULL
		 GROUP BY oi.product_id, oi.name
		 ORDER BY SUM(oi.quantity) DESC LIMIT 10`,
		shopID, startDate)
	defer topRows.Close()

	topProducts := []models.ProductAnalytics{}
	for topRows.Next() {
		var pa models.ProductAnalytics
		topRows.Scan(&pa.ProductID, &pa.Name, &pa.Quantity, &pa.Revenue)
		topProducts = append(topProducts, pa)
	}

	return c.JSON(models.AnalyticsResponse{
		Period:      period,
		Summary:     summary,
		DailyStats:  dailyStats,
		TopProducts: topProducts,
	})
}

func GetRevenueAnalytics(c *fiber.Ctx) error {
	return GetAnalytics(c)
}

func GetProductAnalytics(c *fiber.Ctx) error {
	return GetAnalytics(c)
}

// ========================================
// SETTINGS
// ========================================

func GetSettings(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var settings models.ShopSettings
	err = database.Pool.QueryRow(context.Background(),
		`SELECT id, shop_id, company_name, ico, dic, ic_dph, bank_name, iban, swift,
		        invoice_prefix, invoice_next_number, invoice_footer, tax_rate, prices_include_tax,
		        min_order_value, order_notify_email, low_stock_threshold, terms_url, privacy_url
		 FROM shop_settings WHERE shop_id = $1`, shopID,
	).Scan(&settings.ID, &settings.ShopID, &settings.CompanyName, &settings.ICO, &settings.DIC,
		&settings.ICDPH, &settings.BankName, &settings.IBAN, &settings.SWIFT,
		&settings.InvoicePrefix, &settings.InvoiceNextNumber, &settings.InvoiceFooter,
		&settings.TaxRate, &settings.PricesIncludeTax, &settings.MinOrderValue,
		&settings.OrderNotifyEmail, &settings.LowStockThreshold, &settings.TermsURL, &settings.PrivacyURL)

	if err != nil {
		// Return defaults
		return c.JSON(models.ShopSettings{
			ShopID:            shopID,
			InvoicePrefix:     "FA",
			InvoiceNextNumber: 1,
			TaxRate:           20,
			PricesIncludeTax:  true,
			LowStockThreshold: 5,
		})
	}

	return c.JSON(settings)
}

func UpdateSettings(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req models.ShopSettings
	c.BodyParser(&req)

	database.Pool.Exec(context.Background(),
		`INSERT INTO shop_settings (shop_id, company_name, ico, dic, ic_dph, bank_name, iban, swift,
		                            invoice_prefix, invoice_next_number, invoice_footer, tax_rate, prices_include_tax,
		                            min_order_value, order_notify_email, low_stock_threshold, terms_url, privacy_url)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		 ON CONFLICT (shop_id) DO UPDATE SET
		 company_name = $2, ico = $3, dic = $4, ic_dph = $5, bank_name = $6, iban = $7, swift = $8,
		 invoice_prefix = $9, invoice_footer = $11, tax_rate = $12, prices_include_tax = $13,
		 min_order_value = $14, order_notify_email = $15, low_stock_threshold = $16, terms_url = $17, privacy_url = $18,
		 updated_at = NOW()`,
		shopID, req.CompanyName, req.ICO, req.DIC, req.ICDPH, req.BankName, req.IBAN, req.SWIFT,
		req.InvoicePrefix, req.InvoiceNextNumber, req.InvoiceFooter, req.TaxRate, req.PricesIncludeTax,
		req.MinOrderValue, req.OrderNotifyEmail, req.LowStockThreshold, req.TermsURL, req.PrivacyURL)

	return c.JSON(fiber.Map{"success": true})
}

// ========================================
// INVOICES
// ========================================

func GetInvoices(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	rows, _ := database.Pool.Query(context.Background(),
		`SELECT id, invoice_number, type, issue_date, due_date, status, total, currency, customer_name, order_number
		 FROM invoices WHERE shop_id = $1 ORDER BY created_at DESC LIMIT 100`, shopID)
	defer rows.Close()

	invoices := []map[string]interface{}{}
	for rows.Next() {
		var id uuid.UUID
		var invNum, typ, status, currency string
		var issueDate, dueDate time.Time
		var total float64
		var custName, orderNum *string
		rows.Scan(&id, &invNum, &typ, &issueDate, &dueDate, &status, &total, &currency, &custName, &orderNum)
		invoices = append(invoices, map[string]interface{}{
			"id": id, "invoice_number": invNum, "type": typ, "issue_date": issueDate,
			"due_date": dueDate, "status": status, "total": total, "currency": currency,
			"customer_name": custName, "order_number": orderNum,
		})
	}

	return c.JSON(fiber.Map{"invoices": invoices})
}

func GetInvoice(c *fiber.Ctx) error {
	shopID, _ := uuid.Parse(c.Params("shopId"))
	invID, _ := uuid.Parse(c.Params("id"))
	_, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var inv models.Invoice
	err = database.Pool.QueryRow(context.Background(),
		`SELECT id, shop_id, invoice_number, type, issue_date, due_date, paid_at, status,
		        subtotal, tax, total, currency,
		        supplier_name, supplier_address, supplier_city, supplier_zip, supplier_country,
		        supplier_ico, supplier_dic, supplier_ic_dph, supplier_iban,
		        customer_name, customer_address, customer_city, customer_zip, customer_country,
		        customer_ico, customer_dic, customer_ic_dph, customer_email,
		        items, note, order_id, order_number
		 FROM invoices WHERE id = $1 AND shop_id = $2`, invID, shopID,
	).Scan(&inv.ID, &inv.ShopID, &inv.InvoiceNumber, &inv.Type, &inv.IssueDate, &inv.DueDate,
		&inv.PaidAt, &inv.Status, &inv.Subtotal, &inv.Tax, &inv.Total, &inv.Currency,
		&inv.SupplierName, &inv.SupplierAddress, &inv.SupplierCity, &inv.SupplierZip, &inv.SupplierCountry,
		&inv.SupplierICO, &inv.SupplierDIC, &inv.SupplierICDPH, &inv.SupplierIBAN,
		&inv.CustomerName, &inv.CustomerAddress, &inv.CustomerCity, &inv.CustomerZip, &inv.CustomerCountry,
		&inv.CustomerICO, &inv.CustomerDIC, &inv.CustomerICDPH, &inv.CustomerEmail,
		&inv.Items, &inv.Note, &inv.OrderID, &inv.OrderNumber)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Invoice not found"})
	}

	return c.JSON(inv)
}

func GetInvoicePDF(c *fiber.Ctx) error {
	// For now, return HTML that can be printed as PDF
	return GetInvoice(c)
}
