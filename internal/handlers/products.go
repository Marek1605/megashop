package handlers

import (
	"context"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"eshop-builder/internal/database"
	"eshop-builder/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// GetProducts returns products for a shop
func GetProducts(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	// Pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	search := c.Query("search", "")
	categoryID := c.Query("category_id", "")
	isActive := c.Query("is_active", "")

	// Build query
	whereClause := "WHERE shop_id = $1"
	args := []interface{}{shopID}
	argIndex := 2

	if search != "" {
		whereClause += fmt.Sprintf(" AND (name ILIKE $%d OR sku ILIKE $%d)", argIndex, argIndex)
		args = append(args, "%"+search+"%")
		argIndex++
	}

	if categoryID != "" {
		whereClause += fmt.Sprintf(" AND category_id = $%d", argIndex)
		catID, _ := uuid.Parse(categoryID)
		args = append(args, catID)
		argIndex++
	}

	if isActive == "true" {
		whereClause += " AND is_active = true"
	} else if isActive == "false" {
		whereClause += " AND is_active = false"
	}

	// Get total count
	var total int64
	countQuery := "SELECT COUNT(*) FROM products " + whereClause
	database.Pool.QueryRow(context.Background(), countQuery, args...).Scan(&total)

	// Get products
	query := fmt.Sprintf(`
		SELECT id, shop_id, category_id, name, slug, description, short_description,
		       price, compare_price, cost_price, sku, barcode, quantity,
		       track_inventory, allow_backorder, weight, width, height, length,
		       meta_title, meta_description, is_active, is_featured,
		       created_at, updated_at
		FROM products %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := database.Pool.Query(context.Background(), query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	defer rows.Close()

	products := []models.Product{}
	for rows.Next() {
		var p models.Product
		err := rows.Scan(&p.ID, &p.ShopID, &p.CategoryID, &p.Name, &p.Slug, &p.Description,
			&p.ShortDescription, &p.Price, &p.ComparePrice, &p.CostPrice, &p.SKU, &p.Barcode,
			&p.Quantity, &p.TrackInventory, &p.AllowBackorder, &p.Weight, &p.Width, &p.Height,
			&p.Length, &p.MetaTitle, &p.MetaDescription, &p.IsActive, &p.IsFeatured,
			&p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			continue
		}

		// Get first image
		imgRows, _ := database.Pool.Query(context.Background(),
			`SELECT id, product_id, url, alt, position, created_at 
			 FROM product_images WHERE product_id = $1 ORDER BY position LIMIT 1`,
			p.ID,
		)
		for imgRows.Next() {
			var img models.ProductImage
			imgRows.Scan(&img.ID, &img.ProductID, &img.URL, &img.Alt, &img.Position, &img.CreatedAt)
			p.Images = append(p.Images, img)
		}
		imgRows.Close()

		products = append(products, p)
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return c.JSON(models.PaginatedResponse{
		Data:       products,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

// CreateProduct creates a new product
func CreateProduct(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var req models.CreateProductRequest
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

	// Check slug uniqueness within shop
	var exists bool
	database.Pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM products WHERE shop_id = $1 AND slug = $2)",
		shopID, slug,
	).Scan(&exists)

	if exists {
		slug = slug + "-" + uuid.New().String()[:6]
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	isFeatured := false
	if req.IsFeatured != nil {
		isFeatured = *req.IsFeatured
	}

	ctx := context.Background()

	var product models.Product
	err = database.Pool.QueryRow(ctx,
		`INSERT INTO products (shop_id, category_id, name, slug, description, short_description,
		                       price, compare_price, cost_price, sku, barcode, quantity,
		                       is_active, is_featured, meta_title, meta_description,
		                       created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW(), NOW())
		 RETURNING id, shop_id, category_id, name, slug, description, short_description,
		           price, compare_price, cost_price, sku, barcode, quantity,
		           track_inventory, allow_backorder, weight, width, height, length,
		           meta_title, meta_description, is_active, is_featured,
		           created_at, updated_at`,
		shopID, req.CategoryID, req.Name, slug, req.Description, req.ShortDescription,
		req.Price, req.ComparePrice, req.CostPrice, req.SKU, req.Barcode, req.Quantity,
		isActive, isFeatured, req.MetaTitle, req.MetaDescription,
	).Scan(&product.ID, &product.ShopID, &product.CategoryID, &product.Name, &product.Slug,
		&product.Description, &product.ShortDescription, &product.Price, &product.ComparePrice,
		&product.CostPrice, &product.SKU, &product.Barcode, &product.Quantity,
		&product.TrackInventory, &product.AllowBackorder, &product.Weight, &product.Width,
		&product.Height, &product.Length, &product.MetaTitle, &product.MetaDescription,
		&product.IsActive, &product.IsFeatured, &product.CreatedAt, &product.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create product: " + err.Error()})
	}

	// Add images
	for i, img := range req.Images {
		database.Pool.Exec(ctx,
			`INSERT INTO product_images (product_id, url, alt, position) VALUES ($1, $2, $3, $4)`,
			product.ID, img.URL, img.Alt, i,
		)
	}

	// Load images for response
	imgRows, _ := database.Pool.Query(ctx,
		`SELECT id, product_id, url, alt, position, created_at 
		 FROM product_images WHERE product_id = $1 ORDER BY position`,
		product.ID,
	)
	for imgRows.Next() {
		var img models.ProductImage
		imgRows.Scan(&img.ID, &img.ProductID, &img.URL, &img.Alt, &img.Position, &img.CreatedAt)
		product.Images = append(product.Images, img)
	}
	imgRows.Close()

	return c.Status(fiber.StatusCreated).JSON(product)
}

// GetProduct returns a single product
func GetProduct(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	productID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid product ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	ctx := context.Background()

	var p models.Product
	err = database.Pool.QueryRow(ctx,
		`SELECT id, shop_id, category_id, name, slug, description, short_description,
		        price, compare_price, cost_price, sku, barcode, quantity,
		        track_inventory, allow_backorder, weight, width, height, length,
		        meta_title, meta_description, is_active, is_featured,
		        created_at, updated_at
		 FROM products WHERE id = $1 AND shop_id = $2`,
		productID, shopID,
	).Scan(&p.ID, &p.ShopID, &p.CategoryID, &p.Name, &p.Slug, &p.Description,
		&p.ShortDescription, &p.Price, &p.ComparePrice, &p.CostPrice, &p.SKU, &p.Barcode,
		&p.Quantity, &p.TrackInventory, &p.AllowBackorder, &p.Weight, &p.Width, &p.Height,
		&p.Length, &p.MetaTitle, &p.MetaDescription, &p.IsActive, &p.IsFeatured,
		&p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	// Load images
	imgRows, _ := database.Pool.Query(ctx,
		`SELECT id, product_id, url, alt, position, created_at 
		 FROM product_images WHERE product_id = $1 ORDER BY position`,
		p.ID,
	)
	for imgRows.Next() {
		var img models.ProductImage
		imgRows.Scan(&img.ID, &img.ProductID, &img.URL, &img.Alt, &img.Position, &img.CreatedAt)
		p.Images = append(p.Images, img)
	}
	imgRows.Close()

	// Load variants
	varRows, _ := database.Pool.Query(ctx,
		`SELECT id, product_id, name, sku, price, quantity, options, created_at, updated_at
		 FROM product_variants WHERE product_id = $1`,
		p.ID,
	)
	for varRows.Next() {
		var v models.ProductVariant
		varRows.Scan(&v.ID, &v.ProductID, &v.Name, &v.SKU, &v.Price, &v.Quantity, &v.Options, &v.CreatedAt, &v.UpdatedAt)
		p.Variants = append(p.Variants, v)
	}
	varRows.Close()

	return c.JSON(p)
}

// UpdateProduct updates a product
func UpdateProduct(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	productID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid product ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	allowedFields := map[string]bool{
		"name": true, "slug": true, "description": true, "short_description": true,
		"price": true, "compare_price": true, "cost_price": true,
		"sku": true, "barcode": true, "quantity": true,
		"track_inventory": true, "allow_backorder": true,
		"weight": true, "width": true, "height": true, "length": true,
		"category_id": true, "is_active": true, "is_featured": true,
		"meta_title": true, "meta_description": true,
	}

	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	for field, value := range updates {
		if allowedFields[field] {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", field, argIndex))
			args = append(args, value)
			argIndex++
		}
	}

	if len(setClauses) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No valid fields to update"})
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	args = append(args, productID, shopID)

	query := fmt.Sprintf("UPDATE products SET %s WHERE id = $%d AND shop_id = $%d",
		strings.Join(setClauses, ", "), argIndex, argIndex+1)

	_, err = database.Pool.Exec(context.Background(), query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update product"})
	}

	// Handle images if provided
	if images, ok := updates["images"].([]interface{}); ok {
		ctx := context.Background()
		
		// Delete existing images
		database.Pool.Exec(ctx, "DELETE FROM product_images WHERE product_id = $1", productID)
		
		// Add new images
		for i, img := range images {
			if imgMap, ok := img.(map[string]interface{}); ok {
				url, _ := imgMap["url"].(string)
				alt, _ := imgMap["alt"].(string)
				database.Pool.Exec(ctx,
					`INSERT INTO product_images (product_id, url, alt, position) VALUES ($1, $2, $3, $4)`,
					productID, url, alt, i,
				)
			}
		}
	}

	return c.JSON(fiber.Map{"success": true})
}

// DeleteProduct deletes a product
func DeleteProduct(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	productID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid product ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	_, err = database.Pool.Exec(context.Background(),
		"DELETE FROM products WHERE id = $1 AND shop_id = $2",
		productID, shopID,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete product"})
	}

	return c.JSON(fiber.Map{"success": true})
}

// GetPublicProducts returns products for public storefront
func GetPublicProducts(c *fiber.Ctx) error {
	slug := c.Params("slug")

	var shopID uuid.UUID
	err := database.Pool.QueryRow(context.Background(),
		"SELECT id FROM shops WHERE slug = $1 AND is_published = true AND is_active = true",
		slug,
	).Scan(&shopID)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	// Pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset := (page - 1) * limit

	categorySlug := c.Query("category", "")
	featured := c.Query("featured", "")
	search := c.Query("search", "")

	whereClause := "WHERE p.shop_id = $1 AND p.is_active = true"
	args := []interface{}{shopID}
	argIndex := 2

	if categorySlug != "" {
		whereClause += fmt.Sprintf(" AND c.slug = $%d", argIndex)
		args = append(args, categorySlug)
		argIndex++
	}

	if featured == "true" {
		whereClause += " AND p.is_featured = true"
	}

	if search != "" {
		whereClause += fmt.Sprintf(" AND (p.name ILIKE $%d OR p.description ILIKE $%d)", argIndex, argIndex)
		args = append(args, "%"+search+"%")
		argIndex++
	}

	// Count
	var total int64
	database.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM products p LEFT JOIN categories c ON p.category_id = c.id "+whereClause,
		args...,
	).Scan(&total)

	// Get products
	args = append(args, limit, offset)
	rows, err := database.Pool.Query(context.Background(),
		fmt.Sprintf(`SELECT p.id, p.name, p.slug, p.short_description, p.price, p.compare_price, p.quantity
		 FROM products p
		 LEFT JOIN categories c ON p.category_id = c.id
		 %s ORDER BY p.is_featured DESC, p.created_at DESC
		 LIMIT $%d OFFSET $%d`, whereClause, argIndex, argIndex+1),
		args...,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	defer rows.Close()

	type PublicProduct struct {
		ID               uuid.UUID `json:"id"`
		Name             string    `json:"name"`
		Slug             string    `json:"slug"`
		ShortDescription *string   `json:"short_description"`
		Price            float64   `json:"price"`
		ComparePrice     *float64  `json:"compare_price"`
		Quantity         int       `json:"quantity"`
		Image            *string   `json:"image"`
	}

	products := []PublicProduct{}
	for rows.Next() {
		var p PublicProduct
		rows.Scan(&p.ID, &p.Name, &p.Slug, &p.ShortDescription, &p.Price, &p.ComparePrice, &p.Quantity)

		// Get first image
		var imgURL *string
		database.Pool.QueryRow(context.Background(),
			"SELECT url FROM product_images WHERE product_id = $1 ORDER BY position LIMIT 1",
			p.ID,
		).Scan(&imgURL)
		p.Image = imgURL

		products = append(products, p)
	}

	return c.JSON(fiber.Map{
		"products": products,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// GetPublicProduct returns a single product for storefront
func GetPublicProduct(c *fiber.Ctx) error {
	shopSlug := c.Params("slug")
	productSlug := c.Params("productSlug")

	var shopID uuid.UUID
	err := database.Pool.QueryRow(context.Background(),
		"SELECT id FROM shops WHERE slug = $1 AND is_published = true",
		shopSlug,
	).Scan(&shopID)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	ctx := context.Background()

	var p models.Product
	err = database.Pool.QueryRow(ctx,
		`SELECT id, name, slug, description, short_description, price, compare_price,
		        quantity, is_featured
		 FROM products WHERE shop_id = $1 AND slug = $2 AND is_active = true`,
		shopID, productSlug,
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.ShortDescription, &p.Price,
		&p.ComparePrice, &p.Quantity, &p.IsFeatured)

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	// Load images
	imgRows, _ := database.Pool.Query(ctx,
		"SELECT id, url, alt, position FROM product_images WHERE product_id = $1 ORDER BY position",
		p.ID,
	)
	for imgRows.Next() {
		var img models.ProductImage
		imgRows.Scan(&img.ID, &img.URL, &img.Alt, &img.Position)
		p.Images = append(p.Images, img)
	}
	imgRows.Close()

	// Load variants
	varRows, _ := database.Pool.Query(ctx,
		"SELECT id, name, sku, price, quantity, options FROM product_variants WHERE product_id = $1",
		p.ID,
	)
	for varRows.Next() {
		var v models.ProductVariant
		varRows.Scan(&v.ID, &v.Name, &v.SKU, &v.Price, &v.Quantity, &v.Options)
		p.Variants = append(p.Variants, v)
	}
	varRows.Close()

	return c.JSON(p)
}

// ExportProducts exports products as CSV or XML
func ExportProducts(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	shop, err := verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	format := c.Query("format", "csv")

	rows, err := database.Pool.Query(context.Background(),
		`SELECT id, name, slug, description, short_description, price, compare_price,
		        sku, barcode, quantity, is_active, is_featured, meta_title, meta_description
		 FROM products WHERE shop_id = $1 ORDER BY created_at DESC`,
		shopID,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	defer rows.Close()

	type ExportProduct struct {
		ID               uuid.UUID
		Name             string
		Slug             string
		Description      *string
		ShortDescription *string
		Price            float64
		ComparePrice     *float64
		SKU              *string
		Barcode          *string
		Quantity         int
		IsActive         bool
		IsFeatured       bool
		MetaTitle        *string
		MetaDescription  *string
	}

	products := []ExportProduct{}
	for rows.Next() {
		var p ExportProduct
		rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.ShortDescription, &p.Price,
			&p.ComparePrice, &p.SKU, &p.Barcode, &p.Quantity, &p.IsActive, &p.IsFeatured,
			&p.MetaTitle, &p.MetaDescription)
		products = append(products, p)
	}

	if format == "xml" {
		type XMLProduct struct {
			XMLName xml.Name `xml:"product"`
			ID      string   `xml:"id"`
			Name    string   `xml:"name"`
			Price   float64  `xml:"price"`
			SKU     string   `xml:"sku"`
		}

		type XMLExport struct {
			XMLName  xml.Name     `xml:"products"`
			Products []XMLProduct `xml:"product"`
		}

		export := XMLExport{}
		for _, p := range products {
			sku := ""
			if p.SKU != nil {
				sku = *p.SKU
			}
			export.Products = append(export.Products, XMLProduct{
				ID:    p.ID.String(),
				Name:  p.Name,
				Price: p.Price,
				SKU:   sku,
			})
		}

		c.Set("Content-Type", "application/xml")
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-products.xml", shop.Slug))
		return c.XML(export)
	}

	// CSV export
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-products.csv", shop.Slug))

	writer := csv.NewWriter(c.Response().BodyWriter())
	writer.Comma = ';'

	// Header
	writer.Write([]string{"ID", "Názov", "Slug", "Cena", "SKU", "Množstvo", "Aktívny"})

	for _, p := range products {
		sku := ""
		if p.SKU != nil {
			sku = *p.SKU
		}
		active := "Nie"
		if p.IsActive {
			active = "Áno"
		}
		writer.Write([]string{
			p.ID.String(),
			p.Name,
			p.Slug,
			fmt.Sprintf("%.2f", p.Price),
			sku,
			strconv.Itoa(p.Quantity),
			active,
		})
	}

	writer.Flush()
	return nil
}

// ImportProducts imports products from CSV
func ImportProducts(c *fiber.Ctx) error {
	shopID, err := uuid.Parse(c.Params("shopId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid shop ID"})
	}

	_, err = verifyShopOwnership(c, shopID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Shop not found"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File is required"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open file"})
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = ';'
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid CSV file"})
	}

	if len(records) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "CSV file is empty"})
	}

	created := 0
	errors := []string{}
	ctx := context.Background()

	for i, record := range records[1:] {
		if len(record) < 2 {
			errors = append(errors, fmt.Sprintf("Row %d: insufficient columns", i+2))
			continue
		}

		name := record[0]
		if name == "" {
			errors = append(errors, fmt.Sprintf("Row %d: name is required", i+2))
			continue
		}

		price := 0.0
		if len(record) > 1 {
			price, _ = strconv.ParseFloat(strings.Replace(record[1], ",", ".", 1), 64)
		}

		slug := generateSlug(name) + "-" + uuid.New().String()[:6]

		sku := ""
		if len(record) > 2 {
			sku = record[2]
		}

		quantity := 0
		if len(record) > 3 {
			quantity, _ = strconv.Atoi(record[3])
		}

		_, err := database.Pool.Exec(ctx,
			`INSERT INTO products (shop_id, name, slug, price, sku, quantity, is_active, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, true, NOW(), NOW())`,
			shopID, name, slug, price, sku, quantity,
		)

		if err != nil {
			errors = append(errors, fmt.Sprintf("Row %d: %s", i+2, err.Error()))
		} else {
			created++
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"created": created,
		"errors":  errors,
	})
}
