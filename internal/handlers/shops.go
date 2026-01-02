package handlers

import (
	"context"
	"regexp"
	"strings"

	"eshop-builder/internal/database"
	"eshop-builder/internal/middleware"
	"eshop-builder/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// GetShops returns all shops for the current user
func GetShops(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	rows, err := database.Pool.Query(context.Background(),
		`SELECT id, user_id, name, slug, description, logo, currency, language, 
		        primary_color, email, phone, address, facebook, instagram,
		        meta_title, meta_description, is_active, is_published, custom_domain,
		        created_at, updated_at
		 FROM shops WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	defer rows.Close()

	shops := []models.Shop{}
	for rows.Next() {
		var shop models.Shop
		err := rows.Scan(&shop.ID, &shop.UserID, &shop.Name, &shop.Slug, &shop.Description, &shop.Logo,
			&shop.Currency, &shop.Language, &shop.PrimaryColor, &shop.Email, &shop.Phone,
			&shop.Address, &shop.Facebook, &shop.Instagram, &shop.MetaTitle, &shop.MetaDescription,
			&shop.IsActive, &shop.IsPublished, &shop.CustomDomain, &shop.CreatedAt, &shop.UpdatedAt)
		if err != nil {
			continue
		}
		shops = append(shops, shop)
	}

	return c.JSON(fiber.Map{"shops": shops})
}

// CreateShop creates a new shop
func CreateShop(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req models.CreateShopRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Name is required"})
	}

	// Generate slug
	slug := req.Slug
	if slug == "" {
		slug = generateSlug(req.Name)
	}

	// Check slug uniqueness
	var exists bool
	database.Pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM shops WHERE slug = $1)",
		slug,
	).Scan(&exists)

	if exists {
		slug = slug + "-" + uuid.New().String()[:6]
	}

	currency := req.Currency
	if currency == "" {
		currency = "EUR"
	}

	language := req.Language
	if language == "" {
		language = "sk"
	}

	var shop models.Shop
	err := database.Pool.QueryRow(context.Background(),
		`INSERT INTO shops (user_id, name, slug, description, currency, language, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		 RETURNING id, user_id, name, slug, description, logo, currency, language, 
		           primary_color, email, phone, address, facebook, instagram,
		           meta_title, meta_description, is_active, is_published, custom_domain,
		           created_at, updated_at`,
		userID, req.Name, slug, req.Description, currency, language,
	).Scan(&shop.ID, &shop.UserID, &shop.Name, &shop.Slug, &shop.Description, &shop.Logo,
		&shop.Currency, &shop.Language, &shop.PrimaryColor, &shop.Email, &shop.Phone,
		&shop.Address, &shop.Facebook, &shop.Instagram, &shop.MetaTitle, &shop.MetaDescription,
		&shop.IsActive, &shop.IsPublished, &shop.CustomDomain, &shop.CreatedAt, &shop.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create shop"})
	}

	// Create default settings
	database.Pool.Exec(context.Background(),
		`INSERT INTO shop_settings (shop_id, invoice_prefix, invoice_next_number, tax_rate, prices_include_tax, low_stock_threshold)
		 VALUES ($1, 'FA', 1, 20, true, 5)`,
		shop.ID,
	)

	return c.Status(fiber.StatusCreated).JSON(shop)
}

// GetShop returns a single shop
func GetShop(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	shop, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	return c.JSON(shop)
}

// UpdateShop updates a shop
func UpdateShop(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Build dynamic update query
	allowedFields := map[string]bool{
		"name": true, "slug": true, "description": true, "logo": true,
		"currency": true, "language": true, "primary_color": true,
		"email": true, "phone": true, "address": true,
		"facebook": true, "instagram": true,
		"meta_title": true, "meta_description": true,
		"is_active": true, "is_published": true, "custom_domain": true,
	}

	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	for field, value := range updates {
		if allowedFields[field] {
			setClauses = append(setClauses, field+" = $"+string(rune('0'+argIndex)))
			args = append(args, value)
			argIndex++
		}
	}

	if len(setClauses) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No valid fields to update"})
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	args = append(args, shopID)

	query := "UPDATE shops SET " + strings.Join(setClauses, ", ") + " WHERE id = $" + string(rune('0'+argIndex))

	_, err = database.Pool.Exec(context.Background(), query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update shop"})
	}

	// Return updated shop
	shop, _ := verifyShopOwnership(c, shopID)
	return c.JSON(shop)
}

// DeleteShop deletes a shop
func DeleteShop(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	_, err = database.Pool.Exec(context.Background(), "DELETE FROM shops WHERE id = $1", shopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete shop"})
	}

	return c.JSON(fiber.Map{"success": true})
}

// GetShopStats returns shop statistics
func GetShopStats(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	ctx := context.Background()

	var stats struct {
		TotalProducts   int     `json:"total_products"`
		TotalOrders     int     `json:"total_orders"`
		TotalCustomers  int     `json:"total_customers"`
		TotalRevenue    float64 `json:"total_revenue"`
		PendingOrders   int     `json:"pending_orders"`
		MonthlyRevenue  float64 `json:"monthly_revenue"`
		MonthlyOrders   int     `json:"monthly_orders"`
	}

	database.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM products WHERE shop_id = $1", shopID,
	).Scan(&stats.TotalProducts)

	database.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM orders WHERE shop_id = $1", shopID,
	).Scan(&stats.TotalOrders)

	database.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM customers WHERE shop_id = $1", shopID,
	).Scan(&stats.TotalCustomers)

	database.Pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(total), 0) FROM orders WHERE shop_id = $1 AND payment_status IN ('paid', 'completed')", shopID,
	).Scan(&stats.TotalRevenue)

	database.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM orders WHERE shop_id = $1 AND status = 'pending'", shopID,
	).Scan(&stats.PendingOrders)

	database.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total), 0), COUNT(*) FROM orders 
		 WHERE shop_id = $1 AND payment_status IN ('paid', 'completed')
		 AND created_at >= date_trunc('month', CURRENT_DATE)`, shopID,
	).Scan(&stats.MonthlyRevenue, &stats.MonthlyOrders)

	return c.JSON(stats)
}

// GetPublicShop returns public shop info (no auth required)
func GetPublicShop(c *fiber.Ctx) error {
	slug := c.Params("slug")

	var shop models.Shop
	err := database.Pool.QueryRow(context.Background(),
		`SELECT id, user_id, name, slug, description, logo, currency, language, 
		        primary_color, email, phone, address, facebook, instagram,
		        meta_title, meta_description, is_active, is_published, custom_domain,
		        created_at, updated_at
		 FROM shops WHERE slug = $1 AND is_published = true AND is_active = true`,
		slug,
	).Scan(&shop.ID, &shop.UserID, &shop.Name, &shop.Slug, &shop.Description, &shop.Logo,
		&shop.Currency, &shop.Language, &shop.PrimaryColor, &shop.Email, &shop.Phone,
		&shop.Address, &shop.Facebook, &shop.Instagram, &shop.MetaTitle, &shop.MetaDescription,
		&shop.IsActive, &shop.IsPublished, &shop.CustomDomain, &shop.CreatedAt, &shop.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	// Don't expose user_id publicly
	shop.UserID = uuid.Nil

	return c.JSON(shop)
}

// Helper to generate slug from name
func generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	
	// Replace diacritics
	replacer := strings.NewReplacer(
		"á", "a", "ä", "a", "č", "c", "ď", "d", "é", "e", "ě", "e",
		"í", "i", "ľ", "l", "ĺ", "l", "ň", "n", "ó", "o", "ô", "o",
		"ö", "o", "ř", "r", "š", "s", "ť", "t", "ú", "u", "ů", "u",
		"ü", "u", "ý", "y", "ž", "z",
	)
	slug = replacer.Replace(slug)
	
	// Replace non-alphanumeric with dash
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	
	// Trim dashes
	slug = strings.Trim(slug, "-")
	
	return slug
}
