package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"eshopbuilder/internal/config"
	"eshopbuilder/internal/importer"
	"eshopbuilder/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db            *pgxpool.Pool
	cfg           *config.Config
	importEngines sync.Map // feedID -> *importer.ImportEngine
}

func New(db *pgxpool.Pool, cfg *config.Config) *Handler {
	return &Handler{
		db:  db,
		cfg: cfg,
	}
}

// JSON helper
func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) error(w http.ResponseWriter, status int, message string) {
	h.json(w, status, map[string]string{"error": message})
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// AUTH HANDLERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token     string       `json:"token"`
	ExpiresAt int64        `json:"expires_at"`
	User      *models.User `json:"user"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()

	var user models.User
	err := h.db.QueryRow(ctx, `
		SELECT id, email, password_hash, name, role, is_active
		FROM users WHERE email = $1
	`, req.Email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.IsActive)

	if err != nil {
		h.error(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if !user.IsActive {
		h.error(w, http.StatusUnauthorized, "Account is disabled")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		h.error(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Update last login
	h.db.Exec(ctx, "UPDATE users SET last_login = NOW() WHERE id = $1", user.ID)

	// Generate JWT
	expiresAt := time.Now().Add(24 * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.ID,
		"role": user.Role,
		"exp":  expiresAt.Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Token generation failed")
		return
	}

	h.json(w, http.StatusOK, AuthResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt.Unix(),
		User:      &user,
	})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Password hashing failed")
		return
	}

	ctx := r.Context()
	var userID string
	err = h.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name, role)
		VALUES ($1, $2, $3, 'admin')
		RETURNING id
	`, req.Email, string(hash), req.Name).Scan(&userID)

	if err != nil {
		h.error(w, http.StatusConflict, "Email already exists")
		return
	}

	h.json(w, http.StatusCreated, map[string]string{"id": userID})
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userID := r.Context().Value("user_id").(string)

	var user models.User
	err := h.db.QueryRow(r.Context(), `
		SELECT id, email, name, role FROM users WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.Name, &user.Role)

	if err != nil {
		h.error(w, http.StatusUnauthorized, "User not found")
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.ID,
		"role": user.Role,
		"exp":  expiresAt.Unix(),
	})

	tokenString, _ := token.SignedString([]byte(h.cfg.JWTSecret))

	h.json(w, http.StatusOK, AuthResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt.Unix(),
		User:      &user,
	})
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PRODUCTS HANDLERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type ProductsResponse struct {
	Products   []models.Product `json:"products"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PerPage    int              `json:"per_page"`
	TotalPages int              `json:"total_pages"`
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 24
	}
	offset := (page - 1) * perPage

	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")
	sort := r.URL.Query().Get("sort")

	// Build query
	query := `SELECT id, slug, title, description, price, regular_price, sale_price, 
		image_url, category_id, brand, stock_status, affiliate_url, button_text
		FROM products WHERE is_active = true`
	countQuery := "SELECT COUNT(*) FROM products WHERE is_active = true"
	args := []interface{}{}
	argCount := 1

	if category != "" {
		query += " AND category_id = $" + strconv.Itoa(argCount)
		countQuery += " AND category_id = $" + strconv.Itoa(argCount)
		args = append(args, category)
		argCount++
	}

	if search != "" {
		query += " AND title ILIKE $" + strconv.Itoa(argCount)
		countQuery += " AND title ILIKE $" + strconv.Itoa(argCount)
		args = append(args, "%"+search+"%")
		argCount++
	}

	// Order
	switch sort {
	case "price_asc":
		query += " ORDER BY price ASC"
	case "price_desc":
		query += " ORDER BY price DESC"
	case "name":
		query += " ORDER BY title ASC"
	case "newest":
		query += " ORDER BY created_at DESC"
	default:
		query += " ORDER BY created_at DESC"
	}

	query += " LIMIT $" + strconv.Itoa(argCount) + " OFFSET $" + strconv.Itoa(argCount+1)
	args = append(args, perPage, offset)

	// Get total count
	var total int
	h.db.QueryRow(ctx, countQuery, args[:len(args)-2]...).Scan(&total)

	// Get products
	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	products := []models.Product{}
	for rows.Next() {
		var p models.Product
		rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.Price, &p.RegularPrice,
			&p.SalePrice, &p.ImageURL, &p.CategoryID, &p.Brand, &p.StockStatus,
			&p.AffiliateURL, &p.ButtonText)
		products = append(products, p)
	}

	totalPages := (total + perPage - 1) / perPage

	h.json(w, http.StatusOK, ProductsResponse{
		Products:   products,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	var p models.Product
	err := h.db.QueryRow(ctx, `
		SELECT id, slug, title, description, short_description, price, regular_price, sale_price,
			ean, sku, image_url, gallery_images, category_id, category_path, brand, manufacturer,
			stock_status, stock_quantity, attributes, affiliate_url, button_text, delivery_time
		FROM products WHERE slug = $1 AND is_active = true
	`, slug).Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.ShortDescription, &p.Price,
		&p.RegularPrice, &p.SalePrice, &p.EAN, &p.SKU, &p.ImageURL, &p.GalleryImages,
		&p.CategoryID, &p.CategoryPath, &p.Brand, &p.Manufacturer, &p.StockStatus,
		&p.StockQuantity, &p.Attributes, &p.AffiliateURL, &p.ButtonText, &p.DeliveryTime)

	if err != nil {
		h.error(w, http.StatusNotFound, "Product not found")
		return
	}

	// Increment view count
	h.db.Exec(ctx, "UPDATE products SET view_count = view_count + 1 WHERE id = $1", p.ID)

	// Get category if exists
	if p.CategoryID != nil {
		var cat models.Category
		err := h.db.QueryRow(ctx, `
			SELECT id, name, slug FROM categories WHERE id = $1
		`, *p.CategoryID).Scan(&cat.ID, &cat.Name, &cat.Slug)
		if err == nil {
			p.Category = &cat
		}
	}

	h.json(w, http.StatusOK, p)
}

func (h *Handler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.json(w, http.StatusOK, []models.Product{})
		return
	}

	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id, slug, title, price, image_url
		FROM products
		WHERE is_active = true AND title ILIKE $1
		ORDER BY title
		LIMIT 10
	`, "%"+query+"%")

	if err != nil {
		h.error(w, http.StatusInternalServerError, "Search failed")
		return
	}
	defer rows.Close()

	products := []models.Product{}
	for rows.Next() {
		var p models.Product
		rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Price, &p.ImageURL)
		products = append(products, p)
	}

	h.json(w, http.StatusOK, products)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ADMIN PRODUCTS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (h *Handler) AdminListProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	feedID := r.URL.Query().Get("feed_id")

	query := `SELECT id, slug, title, price, image_url, stock_status, is_active, 
		created_at, updated_at, feed_id FROM products WHERE 1=1`
	countQuery := "SELECT COUNT(*) FROM products WHERE 1=1"
	args := []interface{}{}
	argCount := 1

	if search != "" {
		query += " AND title ILIKE $" + strconv.Itoa(argCount)
		countQuery += " AND title ILIKE $" + strconv.Itoa(argCount)
		args = append(args, "%"+search+"%")
		argCount++
	}

	if status == "active" {
		query += " AND is_active = true"
		countQuery += " AND is_active = true"
	} else if status == "inactive" {
		query += " AND is_active = false"
		countQuery += " AND is_active = false"
	}

	if feedID != "" {
		query += " AND feed_id = $" + strconv.Itoa(argCount)
		countQuery += " AND feed_id = $" + strconv.Itoa(argCount)
		args = append(args, feedID)
		argCount++
	}

	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(argCount) + " OFFSET $" + strconv.Itoa(argCount+1)
	args = append(args, perPage, offset)

	var total int
	h.db.QueryRow(ctx, countQuery, args[:len(args)-2]...).Scan(&total)

	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	products := []models.Product{}
	for rows.Next() {
		var p models.Product
		rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Price, &p.ImageURL, &p.StockStatus,
			&p.IsActive, &p.CreatedAt, &p.UpdatedAt, &p.FeedID)
		products = append(products, p)
	}

	totalPages := (total + perPage - 1) / perPage
	h.json(w, http.StatusOK, ProductsResponse{
		Products:   products,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}

func (h *Handler) AdminGetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	var p models.Product
	err := h.db.QueryRow(ctx, `SELECT * FROM products WHERE id = $1`, id).Scan(
		&p.ID, &p.Slug, &p.Title, &p.Description, &p.ShortDescription,
		&p.Price, &p.RegularPrice, &p.SalePrice, &p.Currency, &p.EAN, &p.SKU,
		&p.MPN, &p.ExternalID, &p.ImageURL, &p.GalleryImages, &p.CategoryID,
		&p.CategoryPath, &p.Brand, &p.Manufacturer, &p.StockStatus, &p.StockQuantity,
		&p.IsActive, &p.IsFeatured, &p.Attributes, &p.AffiliateURL, &p.ButtonText,
		&p.DeliveryTime, &p.FeedID, &p.FeedChecksum, &p.ViewCount, &p.ClickCount,
		&p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		h.error(w, http.StatusNotFound, "Product not found")
		return
	}

	h.json(w, http.StatusOK, p)
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var p models.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	p.ID = uuid.New().String()
	p.Slug = generateSlug(p.Title)

	_, err := h.db.Exec(ctx, `
		INSERT INTO products (id, slug, title, description, short_description, price, 
			regular_price, sale_price, ean, sku, image_url, gallery_images, category_id, 
			brand, manufacturer, stock_status, stock_quantity, is_active, attributes,
			affiliate_url, button_text, delivery_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`, p.ID, p.Slug, p.Title, p.Description, p.ShortDescription, p.Price,
		p.RegularPrice, p.SalePrice, p.EAN, p.SKU, p.ImageURL, p.GalleryImages,
		p.CategoryID, p.Brand, p.Manufacturer, p.StockStatus, p.StockQuantity,
		p.IsActive, p.Attributes, p.AffiliateURL, p.ButtonText, p.DeliveryTime)

	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to create product")
		return
	}

	h.json(w, http.StatusCreated, p)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var p models.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	_, err := h.db.Exec(ctx, `
		UPDATE products SET
			title = $2, description = $3, short_description = $4, price = $5,
			regular_price = $6, sale_price = $7, ean = $8, sku = $9, image_url = $10,
			gallery_images = $11, category_id = $12, brand = $13, manufacturer = $14,
			stock_status = $15, stock_quantity = $16, is_active = $17, attributes = $18,
			affiliate_url = $19, button_text = $20, delivery_time = $21, updated_at = NOW()
		WHERE id = $1
	`, id, p.Title, p.Description, p.ShortDescription, p.Price,
		p.RegularPrice, p.SalePrice, p.EAN, p.SKU, p.ImageURL,
		p.GalleryImages, p.CategoryID, p.Brand, p.Manufacturer,
		p.StockStatus, p.StockQuantity, p.IsActive, p.Attributes,
		p.AffiliateURL, p.ButtonText, p.DeliveryTime)

	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to update product")
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	_, err := h.db.Exec(ctx, "DELETE FROM products WHERE id = $1", id)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to delete product")
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) BulkProductAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"` // activate, deactivate, delete
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()

	switch req.Action {
	case "activate":
		h.db.Exec(ctx, "UPDATE products SET is_active = true WHERE id = ANY($1)", req.IDs)
	case "deactivate":
		h.db.Exec(ctx, "UPDATE products SET is_active = false WHERE id = ANY($1)", req.IDs)
	case "delete":
		h.db.Exec(ctx, "DELETE FROM products WHERE id = ANY($1)", req.IDs)
	default:
		h.error(w, http.StatusBadRequest, "Unknown action")
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "done"})
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CATEGORIES HANDLERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id, name, slug, description, image_url, parent_id, product_count, sort_order
		FROM categories
		WHERE is_active = true
		ORDER BY sort_order, name
	`)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var c models.Category
		rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.ImageURL,
			&c.ParentID, &c.ProductCount, &c.SortOrder)
		categories = append(categories, c)
	}

	// Build tree structure
	tree := buildCategoryTree(categories, nil)
	h.json(w, http.StatusOK, tree)
}

func (h *Handler) GetCategory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	var c models.Category
	err := h.db.QueryRow(ctx, `
		SELECT id, name, slug, description, image_url, parent_id, product_count
		FROM categories WHERE slug = $1 AND is_active = true
	`, slug).Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.ImageURL, &c.ParentID, &c.ProductCount)

	if err != nil {
		h.error(w, http.StatusNotFound, "Category not found")
		return
	}

	h.json(w, http.StatusOK, c)
}

func (h *Handler) AdminListCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id, name, slug, description, image_url, parent_id, product_count, sort_order, is_active
		FROM categories ORDER BY sort_order, name
	`)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var c models.Category
		rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.ImageURL,
			&c.ParentID, &c.ProductCount, &c.SortOrder, &c.IsActive)
		categories = append(categories, c)
	}

	h.json(w, http.StatusOK, categories)
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var c models.Category
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	c.ID = uuid.New().String()
	c.Slug = generateSlug(c.Name)

	_, err := h.db.Exec(ctx, `
		INSERT INTO categories (id, name, slug, description, image_url, parent_id, sort_order, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, c.ID, c.Name, c.Slug, c.Description, c.ImageURL, c.ParentID, c.SortOrder, true)

	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to create category")
		return
	}

	h.json(w, http.StatusCreated, c)
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var c models.Category
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	_, err := h.db.Exec(ctx, `
		UPDATE categories SET
			name = $2, description = $3, image_url = $4, parent_id = $5, 
			sort_order = $6, is_active = $7, updated_at = NOW()
		WHERE id = $1
	`, id, c.Name, c.Description, c.ImageURL, c.ParentID, c.SortOrder, c.IsActive)

	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to update category")
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	_, err := h.db.Exec(ctx, "DELETE FROM categories WHERE id = $1", id)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to delete category")
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// DASHBOARD
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (h *Handler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats := make(map[string]interface{})

	h.db.QueryRow(ctx, "SELECT COUNT(*) FROM products WHERE is_active = true").Scan(&stats["total_products"])
	h.db.QueryRow(ctx, "SELECT COUNT(*) FROM categories WHERE is_active = true").Scan(&stats["total_categories"])
	h.db.QueryRow(ctx, "SELECT COUNT(*) FROM feeds WHERE active = true").Scan(&stats["total_feeds"])
	h.db.QueryRow(ctx, "SELECT COALESCE(SUM(view_count), 0) FROM products").Scan(&stats["total_views"])
	h.db.QueryRow(ctx, "SELECT COALESCE(SUM(click_count), 0) FROM products").Scan(&stats["total_clicks"])

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) GetRecentActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id, feed_id, started_at, finished_at, duration, created, updated, skipped, errors, status
		FROM import_history
		ORDER BY started_at DESC
		LIMIT 10
	`)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	history := []models.ImportHistory{}
	for rows.Next() {
		var h models.ImportHistory
		rows.Scan(&h.ID, &h.FeedID, &h.StartedAt, &h.FinishedAt, &h.Duration,
			&h.Created, &h.Updated, &h.Skipped, &h.Errors, &h.Status)
		history = append(history, h)
	}

	h.json(w, http.StatusOK, history)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SHOP CONFIG
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (h *Handler) GetShopConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var config models.ShopConfig
	err := h.db.QueryRow(ctx, `SELECT * FROM shop_config LIMIT 1`).Scan(
		&config.ID, &config.ShopName, &config.ShopURL, &config.Logo, &config.Favicon,
		&config.Currency, &config.Locale, &config.Template, &config.PrimaryColor,
		&config.SecondaryColor, &config.GoogleAnalytics, &config.MetaTitle,
		&config.MetaDescription, &config.CustomCSS, &config.CustomJS, &config.Settings,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err != nil {
		// Create default config
		config = models.ShopConfig{
			ShopName:       "EshopBuilder Store",
			Template:       "aurora",
			Currency:       "EUR",
			Locale:         "sk",
			PrimaryColor:   "#3B82F6",
			SecondaryColor: "#10B981",
		}
	}

	h.json(w, http.StatusOK, config)
}

func (h *Handler) UpdateShopConfig(w http.ResponseWriter, r *http.Request) {
	var config models.ShopConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	_, err := h.db.Exec(ctx, `
		UPDATE shop_config SET
			shop_name = $1, shop_url = $2, logo = $3, favicon = $4,
			currency = $5, locale = $6, template = $7, primary_color = $8,
			secondary_color = $9, google_analytics = $10, meta_title = $11,
			meta_description = $12, custom_css = $13, custom_js = $14,
			settings = $15, updated_at = NOW()
	`, config.ShopName, config.ShopURL, config.Logo, config.Favicon,
		config.Currency, config.Locale, config.Template, config.PrimaryColor,
		config.SecondaryColor, config.GoogleAnalytics, config.MetaTitle,
		config.MetaDescription, config.CustomCSS, config.CustomJS, config.Settings)

	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to update config")
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	h.GetShopConfig(w, r)
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	h.UpdateShopConfig(w, r)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FEED HANDLERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func (h *Handler) ListFeeds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id, name, description, feed_url, feed_type, active, status, 
			last_run, last_error, total_products, created_at
		FROM feeds ORDER BY created_at DESC
	`)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	feeds := []models.Feed{}
	for rows.Next() {
		var f models.Feed
		rows.Scan(&f.ID, &f.Name, &f.Description, &f.FeedURL, &f.FeedType,
			&f.Active, &f.Status, &f.LastRun, &f.LastError, &f.TotalProducts, &f.CreatedAt)
		feeds = append(feeds, f)
	}

	h.json(w, http.StatusOK, feeds)
}

func (h *Handler) GetFeed(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	var f models.Feed
	err := h.db.QueryRow(ctx, `SELECT * FROM feeds WHERE id = $1`, id).Scan(
		&f.ID, &f.Name, &f.Description, &f.FeedURL, &f.FeedType, &f.XMLItemPath,
		&f.CSVDelimiter, &f.CSVHasHeader, &f.ImportMode, &f.MatchBy, &f.DefaultCategory,
		&f.ImportImages, &f.CreateAttributes, &f.ScheduleEnabled, &f.ScheduleCron,
		&f.Active, &f.Status, &f.LastRun, &f.LastError, &f.TotalProducts,
		&f.FieldMappings, &f.Settings, &f.CreatedAt, &f.UpdatedAt,
	)

	if err != nil {
		h.error(w, http.StatusNotFound, "Feed not found")
		return
	}

	h.json(w, http.StatusOK, f)
}

func (h *Handler) CreateFeed(w http.ResponseWriter, r *http.Request) {
	var f models.Feed
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	f.ID = uuid.New().String()
	f.Status = models.FeedStatusActive

	_, err := h.db.Exec(ctx, `
		INSERT INTO feeds (id, name, description, feed_url, feed_type, xml_item_path,
			csv_delimiter, csv_has_header, import_mode, match_by, default_category,
			import_images, create_attributes, schedule_enabled, schedule_cron,
			active, status, field_mappings, settings)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`, f.ID, f.Name, f.Description, f.FeedURL, f.FeedType, f.XMLItemPath,
		f.CSVDelimiter, f.CSVHasHeader, f.ImportMode, f.MatchBy, f.DefaultCategory,
		f.ImportImages, f.CreateAttributes, f.ScheduleEnabled, f.ScheduleCron,
		f.Active, f.Status, f.FieldMappings, f.Settings)

	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to create feed: "+err.Error())
		return
	}

	h.json(w, http.StatusCreated, f)
}

func (h *Handler) UpdateFeed(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var f models.Feed
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	_, err := h.db.Exec(ctx, `
		UPDATE feeds SET
			name = $2, description = $3, feed_url = $4, feed_type = $5, xml_item_path = $6,
			csv_delimiter = $7, csv_has_header = $8, import_mode = $9, match_by = $10,
			default_category = $11, import_images = $12, create_attributes = $13,
			schedule_enabled = $14, schedule_cron = $15, active = $16, field_mappings = $17,
			settings = $18, updated_at = NOW()
		WHERE id = $1
	`, id, f.Name, f.Description, f.FeedURL, f.FeedType, f.XMLItemPath,
		f.CSVDelimiter, f.CSVHasHeader, f.ImportMode, f.MatchBy, f.DefaultCategory,
		f.ImportImages, f.CreateAttributes, f.ScheduleEnabled, f.ScheduleCron,
		f.Active, f.FieldMappings, f.Settings)

	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to update feed")
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) DeleteFeed(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	_, err := h.db.Exec(ctx, "DELETE FROM feeds WHERE id = $1", id)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Failed to delete feed")
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) StartImport(w http.ResponseWriter, r *http.Request) {
	feedID := chi.URLParam(r, "id")
	ctx := r.Context()

	// Get feed
	var feed models.Feed
	err := h.db.QueryRow(ctx, `SELECT * FROM feeds WHERE id = $1`, feedID).Scan(
		&feed.ID, &feed.Name, &feed.Description, &feed.FeedURL, &feed.FeedType, &feed.XMLItemPath,
		&feed.CSVDelimiter, &feed.CSVHasHeader, &feed.ImportMode, &feed.MatchBy, &feed.DefaultCategory,
		&feed.ImportImages, &feed.CreateAttributes, &feed.ScheduleEnabled, &feed.ScheduleCron,
		&feed.Active, &feed.Status, &feed.LastRun, &feed.LastError, &feed.TotalProducts,
		&feed.FieldMappings, &feed.Settings, &feed.CreatedAt, &feed.UpdatedAt,
	)
	if err != nil {
		h.error(w, http.StatusNotFound, "Feed not found")
		return
	}

	// Check if already running
	if _, exists := h.importEngines.Load(feedID); exists {
		h.error(w, http.StatusConflict, "Import already running")
		return
	}

	// Create import engine
	engine := importer.NewImportEngine(h.db, &feed)
	h.importEngines.Store(feedID, engine)

	// Start import in background
	go func() {
		defer h.importEngines.Delete(feedID)
		engine.Run(context.Background(), "manual")
	}()

	h.json(w, http.StatusOK, map[string]string{"status": "started", "feed_id": feedID})
}

func (h *Handler) StopImport(w http.ResponseWriter, r *http.Request) {
	feedID := chi.URLParam(r, "id")

	if engine, ok := h.importEngines.Load(feedID); ok {
		engine.(*importer.ImportEngine).Stop()
		h.json(w, http.StatusOK, map[string]string{"status": "stopping"})
		return
	}

	h.error(w, http.StatusNotFound, "No running import found")
}

func (h *Handler) GetImportProgress(w http.ResponseWriter, r *http.Request) {
	feedID := chi.URLParam(r, "id")

	if engine, ok := h.importEngines.Load(feedID); ok {
		progress := engine.(*importer.ImportEngine).GetProgress()
		h.json(w, http.StatusOK, progress)
		return
	}

	// Return idle status if no import running
	h.json(w, http.StatusOK, models.ImportProgress{
		FeedID: feedID,
		Status: models.ImportStatusIdle,
	})
}

func (h *Handler) GetImportHistory(w http.ResponseWriter, r *http.Request) {
	feedID := chi.URLParam(r, "id")
	ctx := r.Context()

	rows, err := h.db.Query(ctx, `
		SELECT id, feed_id, started_at, finished_at, duration, total_items,
			processed, created, updated, skipped, errors, status, error_message, triggered_by
		FROM import_history
		WHERE feed_id = $1
		ORDER BY started_at DESC
		LIMIT 20
	`, feedID)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	history := []models.ImportHistory{}
	for rows.Next() {
		var h models.ImportHistory
		rows.Scan(&h.ID, &h.FeedID, &h.StartedAt, &h.FinishedAt, &h.Duration,
			&h.TotalItems, &h.Processed, &h.Created, &h.Updated, &h.Skipped,
			&h.Errors, &h.Status, &h.ErrorMessage, &h.TriggeredBy)
		history = append(history, h)
	}

	h.json(w, http.StatusOK, history)
}

func (h *Handler) PreviewFeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL          string `json:"url"`
		Type         string `json:"type"`
		XMLItemPath  string `json:"xml_item_path"`
		CSVDelimiter string `json:"csv_delimiter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	parser := importer.NewFeedParser(req.URL, req.Type)
	if req.XMLItemPath != "" {
		parser.XMLItemPath = req.XMLItemPath
	}
	if req.CSVDelimiter != "" {
		parser.CSVDelimiter = req.CSVDelimiter
	}

	result, err := parser.Preview(10)
	if err != nil {
		h.error(w, http.StatusBadRequest, "Preview failed: "+err.Error())
		return
	}

	h.json(w, http.StatusOK, result)
}

func (h *Handler) AutoMapping(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Fields []string `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	parser := importer.NewFeedParser("", "")
	mappings := parser.AutoDetectMappings(req.Fields)

	h.json(w, http.StatusOK, mappings)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

func buildCategoryTree(categories []models.Category, parentID *string) []*models.Category {
	var tree []*models.Category

	for i := range categories {
		c := &categories[i]
		if (parentID == nil && c.ParentID == nil) || (parentID != nil && c.ParentID != nil && *parentID == *c.ParentID) {
			c.Children = buildCategoryTree(categories, &c.ID)
			tree = append(tree, c)
		}
	}

	return tree
}

func generateSlug(text string) string {
	// Simplified slug generation
	slug := text
	replacements := map[string]string{
		"á": "a", "ä": "a", "č": "c", "ď": "d", "é": "e", "í": "i",
		"ĺ": "l", "ľ": "l", "ň": "n", "ó": "o", "ô": "o", "ŕ": "r",
		"š": "s", "ť": "t", "ú": "u", "ý": "y", "ž": "z", " ": "-",
	}
	for from, to := range replacements {
		slug = replaceAll(slug, from, to)
	}
	return slug
}

func replaceAll(s, old, new string) string {
	result := s
	for i := 0; i < len(result); i++ {
		if i+len(old) <= len(result) && result[i:i+len(old)] == old {
			result = result[:i] + new + result[i+len(old):]
		}
	}
	return result
}
