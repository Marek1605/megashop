package handlers

import (
	"context"
	"strings"
	"time"

	"eshop-builder/internal/database"
	"eshop-builder/internal/middleware"
	"eshop-builder/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Register creates a new user
func Register(c *fiber.Ctx) error {
	var req models.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email and password are required"})
	}

	// Check if user exists
	var exists bool
	err := database.Pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)",
		strings.ToLower(req.Email),
	).Scan(&exists)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	if exists {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already registered"})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	// Create user
	var user models.User
	err = database.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash, name, plan, created_at, updated_at)
		 VALUES ($1, $2, $3, 'FREE', NOW(), NOW())
		 RETURNING id, email, name, plan, created_at, updated_at`,
		strings.ToLower(req.Email), string(hashedPassword), req.Name,
	).Scan(&user.ID, &user.Email, &user.Name, &user.Plan, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	// Generate tokens
	token, err := middleware.GenerateToken(user.ID, user.Email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	refreshToken, err := middleware.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate refresh token"})
	}

	return c.Status(fiber.StatusCreated).JSON(models.AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// Login authenticates a user
func Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email and password are required"})
	}

	var user models.User
	err := database.Pool.QueryRow(context.Background(),
		`SELECT id, email, password_hash, name, plan, created_at, updated_at
		 FROM users WHERE email = $1`,
		strings.ToLower(req.Email),
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Plan, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Generate tokens
	token, err := middleware.GenerateToken(user.ID, user.Email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	refreshToken, err := middleware.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate refresh token"})
	}

	return c.JSON(models.AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// RefreshToken refreshes the access token
func RefreshToken(c *fiber.Ctx) error {
	type RefreshRequest struct {
		RefreshToken string `json:"refresh_token"`
	}

	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	claims, err := middleware.ParseToken(req.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid refresh token"})
	}

	// Generate new tokens
	token, err := middleware.GenerateToken(claims.UserID, claims.Email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	refreshToken, err := middleware.GenerateRefreshToken(claims.UserID, claims.Email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate refresh token"})
	}

	return c.JSON(fiber.Map{
		"token":         token,
		"refresh_token": refreshToken,
	})
}

// GetCurrentUser returns the current user
func GetCurrentUser(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var user models.User
	err := database.Pool.QueryRow(context.Background(),
		`SELECT id, email, name, plan, created_at, updated_at
		 FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.Name, &user.Plan, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(user)
}

// UpdateCurrentUser updates the current user
func UpdateCurrentUser(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	type UpdateRequest struct {
		Name     *string `json:"name"`
		Password *string `json:"password"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Build update query
	updates := []string{}
	args := []interface{}{}
	argIndex := 1

	if req.Name != nil {
		updates = append(updates, "name = $"+string(rune('0'+argIndex)))
		args = append(args, *req.Name)
		argIndex++
	}

	if req.Password != nil && len(*req.Password) >= 6 {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err == nil {
			updates = append(updates, "password_hash = $"+string(rune('0'+argIndex)))
			args = append(args, string(hashedPassword))
			argIndex++
		}
	}

	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No fields to update"})
	}

	updates = append(updates, "updated_at = NOW()")
	args = append(args, userID)

	query := "UPDATE users SET " + strings.Join(updates, ", ") + " WHERE id = $" + string(rune('0'+argIndex))

	_, err := database.Pool.Exec(context.Background(), query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user"})
	}

	// Return updated user
	var user models.User
	err = database.Pool.QueryRow(context.Background(),
		`SELECT id, email, name, plan, created_at, updated_at
		 FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.Name, &user.Plan, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get user"})
	}

	return c.JSON(user)
}

// Helper to verify shop ownership
func verifyShopOwnership(c *fiber.Ctx, shopID uuid.UUID) (*models.Shop, error) {
	userID := middleware.GetUserID(c)
	if userID == uuid.Nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Unauthorized")
	}

	var shop models.Shop
	err := database.Pool.QueryRow(context.Background(),
		`SELECT id, user_id, name, slug, description, logo, currency, language, 
		        primary_color, email, phone, address, facebook, instagram,
		        meta_title, meta_description, is_active, is_published, custom_domain,
		        created_at, updated_at
		 FROM shops WHERE id = $1 AND user_id = $2`,
		shopID, userID,
	).Scan(&shop.ID, &shop.UserID, &shop.Name, &shop.Slug, &shop.Description, &shop.Logo,
		&shop.Currency, &shop.Language, &shop.PrimaryColor, &shop.Email, &shop.Phone,
		&shop.Address, &shop.Facebook, &shop.Instagram, &shop.MetaTitle, &shop.MetaDescription,
		&shop.IsActive, &shop.IsPublished, &shop.CustomDomain, &shop.CreatedAt, &shop.UpdatedAt)

	if err != nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "Shop not found")
	}

	return &shop, nil
}

// Helper to generate order number
func generateOrderNumber() string {
	now := time.Now()
	return "ORD-" + now.Format("0601") + "-" + strings.ToUpper(uuid.New().String()[:6])
}

// Helper to generate invoice number
func generateInvoiceNumber(shopID uuid.UUID) (string, error) {
	ctx := context.Background()
	
	var prefix string
	var nextNumber int
	
	err := database.Pool.QueryRow(ctx,
		`SELECT COALESCE(invoice_prefix, 'FA'), COALESCE(invoice_next_number, 1)
		 FROM shop_settings WHERE shop_id = $1`,
		shopID,
	).Scan(&prefix, &nextNumber)
	
	if err != nil {
		prefix = "FA"
		nextNumber = 1
	}
	
	// Update next number
	database.Pool.Exec(ctx,
		`UPDATE shop_settings SET invoice_next_number = invoice_next_number + 1 WHERE shop_id = $1`,
		shopID,
	)
	
	year := time.Now().Year()
	return prefix + string(rune(year)) + strings.Repeat("0", 6-len(string(rune(nextNumber)))) + string(rune(nextNumber)), nil
}
