package importer

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"eshopbuilder/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ImportEngine - Hlavný import engine
type ImportEngine struct {
	db        *pgxpool.Pool
	feed      *models.Feed
	parser    *FeedParser
	progress  *models.ImportProgress
	historyID string

	isRunning  bool
	shouldStop bool
	mutex      sync.RWMutex
	startTime  time.Time

	categoryCache map[string]string
}

// NewImportEngine vytvorí nový engine
func NewImportEngine(db *pgxpool.Pool, feed *models.Feed) *ImportEngine {
	return &ImportEngine{
		db:            db,
		feed:          feed,
		categoryCache: make(map[string]string),
	}
}

// Run spustí import
func (e *ImportEngine) Run(ctx context.Context, triggeredBy string) (*models.ImportHistory, error) {
	e.mutex.Lock()
	if e.isRunning {
		e.mutex.Unlock()
		return nil, fmt.Errorf("import already running")
	}
	e.isRunning = true
	e.shouldStop = false
	e.mutex.Unlock()

	defer func() {
		e.mutex.Lock()
		e.isRunning = false
		e.mutex.Unlock()
	}()

	// Initialize
	e.historyID = uuid.New().String()
	e.startTime = time.Now()

	e.progress = &models.ImportProgress{
		FeedID:    e.feed.ID,
		HistoryID: e.historyID,
		Status:    models.ImportStatusRunning,
		Message:   "Initializing import...",
		Logs:      []models.LogEntry{},
	}

	history := &models.ImportHistory{
		ID:          e.historyID,
		FeedID:      e.feed.ID,
		StartedAt:   e.startTime,
		Status:      models.ImportStatusRunning,
		TriggeredBy: triggeredBy,
	}

	// Save initial history
	e.saveHistory(ctx, history)
	e.updateFeedStatus(ctx, "running", "")

	e.log("info", "Import started for feed: "+e.feed.Name)
	e.updateProgress("Downloading feed...")

	// Initialize parser
	e.parser = NewFeedParser(e.feed.FeedURL, string(e.feed.FeedType))
	e.parser.XMLItemPath = e.feed.XMLItemPath
	e.parser.CSVDelimiter = e.feed.CSVDelimiter

	// Download feed
	feedData, err := e.parser.Download()
	if err != nil {
		return e.failImport(ctx, history, "Download error: "+err.Error())
	}

	e.log("info", fmt.Sprintf("Feed downloaded: %d KB", len(feedData)/1024))
	e.updateProgress("Parsing feed...")

	// Count total items
	totalCount := 0
	switch e.feed.FeedType {
	case models.FeedTypeXML:
		e.parser.ParseXMLFull(feedData, func(item map[string]interface{}) error {
			totalCount++
			return nil
		})
	case models.FeedTypeCSV:
		e.parser.ParseCSVFull(feedData, func(item map[string]interface{}) error {
			totalCount++
			return nil
		})
	case models.FeedTypeJSON:
		e.parser.ParseJSONFull(feedData, func(item map[string]interface{}) error {
			totalCount++
			return nil
		})
	}

	e.progress.Total = totalCount
	history.TotalItems = totalCount
	e.log("info", fmt.Sprintf("Feed parsed: %d items", totalCount))
	e.updateProgress(fmt.Sprintf("Processing %d items...", totalCount))

	// Process items
	processCallback := func(item map[string]interface{}) error {
		if e.shouldStop {
			return fmt.Errorf("import cancelled")
		}

		e.progress.Processed++

		// Map item
		feedItem := e.mapItem(item)
		if feedItem == nil {
			e.progress.Skipped++
			return nil
		}

		// Validate
		if feedItem.Title == "" {
			e.progress.Errors++
			return nil
		}

		if feedItem.Price == 0 {
			e.progress.Errors++
			return nil
		}

		// Process item
		if err := e.processItem(ctx, feedItem); err != nil {
			e.progress.Errors++
			e.log("error", fmt.Sprintf("Item error (%s): %s", feedItem.Title, err.Error()))
			return nil
		}

		if e.progress.Processed%50 == 0 {
			e.updateProgressStats()
		}

		return nil
	}

	// Parse and process
	switch e.feed.FeedType {
	case models.FeedTypeXML:
		err = e.parser.ParseXMLFull(feedData, processCallback)
	case models.FeedTypeCSV:
		err = e.parser.ParseCSVFull(feedData, processCallback)
	case models.FeedTypeJSON:
		err = e.parser.ParseJSONFull(feedData, processCallback)
	}

	if err != nil && !strings.Contains(err.Error(), "cancelled") {
		return e.failImport(ctx, history, err.Error())
	}

	return e.completeImport(ctx, history)
}

func (e *ImportEngine) mapItem(raw map[string]interface{}) *models.FeedItem {
	item := &models.FeedItem{}

	// Get field mappings
	var mappings []models.FieldMapping
	if e.feed.FieldMappings != nil {
		data, _ := json.Marshal(e.feed.FieldMappings)
		json.Unmarshal(data, &mappings)
	}

	// Apply mappings
	for _, mapping := range mappings {
		value := e.getFieldValue(raw, mapping.SourceField)
		if value == "" && mapping.DefaultValue != "" {
			value = mapping.DefaultValue
		}

		// Apply transform
		value = e.applyTransform(value, mapping.TransformType, mapping.TransformValue)

		// Set target field
		switch mapping.TargetField {
		case "title":
			item.Title = value
		case "description":
			item.Description = value
		case "short_description":
			item.ShortDescription = value
		case "price":
			item.Price = e.parsePrice(value)
		case "regular_price":
			item.RegularPrice = e.parsePrice(value)
		case "sale_price":
			item.SalePrice = e.parsePrice(value)
		case "ean":
			item.EAN = value
		case "sku":
			item.SKU = value
		case "external_id":
			item.ExternalID = value
		case "image_url":
			item.ImageURL = value
		case "gallery_images":
			item.GalleryImages = strings.Split(value, "|")
		case "category":
			item.CategoryPath = value
		case "brand":
			item.Brand = value
		case "manufacturer":
			item.Manufacturer = value
		case "stock_status":
			item.StockStatus = value
		case "stock_quantity":
			item.StockQuantity, _ = strconv.Atoi(value)
		case "affiliate_url":
			item.AffiliateURL = value
		case "button_text":
			item.ButtonText = value
		case "delivery_time":
			item.DeliveryTime = value
		}
	}

	// Auto-map if no mappings
	if len(mappings) == 0 {
		item.Title = e.getFieldValue(raw, "PRODUCTNAME", "title", "name", "nazov")
		item.Description = e.getFieldValue(raw, "DESCRIPTION", "description", "popis")
		item.Price = e.parsePrice(e.getFieldValue(raw, "PRICE_VAT", "price", "cena"))
		item.EAN = e.getFieldValue(raw, "EAN", "ean", "ean13", "gtin")
		item.SKU = e.getFieldValue(raw, "SKU", "sku", "ITEMGROUP_ID", "kod")
		item.ImageURL = e.getFieldValue(raw, "IMGURL", "image", "image_url", "img_url")
		item.CategoryPath = e.getFieldValue(raw, "CATEGORYTEXT", "category", "kategoria")
		item.Brand = e.getFieldValue(raw, "MANUFACTURER", "brand", "vyrobca")
		item.AffiliateURL = e.getFieldValue(raw, "URL", "url", "link")
	}

	if item.Title == "" {
		return nil
	}

	return item
}

func (e *ImportEngine) getFieldValue(raw map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := raw[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		// Try lowercase
		if val, ok := raw[strings.ToLower(key)]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	return ""
}

func (e *ImportEngine) applyTransform(value, transformType, transformValue string) string {
	switch transformType {
	case "trim":
		return strings.TrimSpace(value)
	case "lowercase":
		return strings.ToLower(value)
	case "uppercase":
		return strings.ToUpper(value)
	case "regex":
		parts := strings.Split(transformValue, "|||")
		if len(parts) == 2 {
			re, _ := regexp.Compile(parts[0])
			return re.ReplaceAllString(value, parts[1])
		}
	case "default":
		if value == "" {
			return transformValue
		}
	}
	return value
}

func (e *ImportEngine) parsePrice(value string) float64 {
	// Remove currency symbols
	value = regexp.MustCompile(`[^\d,.]`).ReplaceAllString(value, "")
	// Normalize decimals
	value = strings.ReplaceAll(value, ",", ".")
	price, _ := strconv.ParseFloat(value, 64)
	return price
}

func (e *ImportEngine) processItem(ctx context.Context, item *models.FeedItem) error {
	// Find existing product
	existingID, checksum := e.findExistingProduct(ctx, item)

	// Calculate new checksum
	newChecksum := e.calculateChecksum(item)

	// Skip unchanged
	if existingID != "" && checksum == newChecksum {
		e.progress.Skipped++
		return nil
	}

	// Get or create category
	var categoryID *string
	if item.CategoryPath != "" {
		categoryID = e.getOrCreateCategory(ctx, item.CategoryPath)
	}

	if existingID != "" {
		// Update
		if err := e.updateProduct(ctx, existingID, item, categoryID, newChecksum); err != nil {
			return err
		}
		e.progress.Updated++
	} else {
		// Create
		if err := e.createProduct(ctx, item, categoryID, newChecksum); err != nil {
			return err
		}
		e.progress.Created++
	}

	return nil
}

func (e *ImportEngine) findExistingProduct(ctx context.Context, item *models.FeedItem) (string, string) {
	var id, checksum string

	switch e.feed.MatchBy {
	case models.MatchByEAN:
		if item.EAN != "" {
			e.db.QueryRow(ctx, "SELECT id, feed_checksum FROM products WHERE ean = $1 LIMIT 1", item.EAN).Scan(&id, &checksum)
		}
	case models.MatchBySKU:
		if item.SKU != "" {
			e.db.QueryRow(ctx, "SELECT id, feed_checksum FROM products WHERE sku = $1 LIMIT 1", item.SKU).Scan(&id, &checksum)
		}
	case models.MatchByExternalID:
		if item.ExternalID != "" {
			e.db.QueryRow(ctx, "SELECT id, feed_checksum FROM products WHERE external_id = $1 LIMIT 1", item.ExternalID).Scan(&id, &checksum)
		}
	case models.MatchByTitle:
		e.db.QueryRow(ctx, "SELECT id, feed_checksum FROM products WHERE title = $1 LIMIT 1", item.Title).Scan(&id, &checksum)
	}

	return id, checksum
}

func (e *ImportEngine) calculateChecksum(item *models.FeedItem) string {
	data := fmt.Sprintf("%s|%s|%s|%.2f|%s|%s",
		item.Title, item.Description, item.EAN, item.Price, item.ImageURL, item.CategoryPath)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (e *ImportEngine) createProduct(ctx context.Context, item *models.FeedItem, categoryID *string, checksum string) error {
	id := uuid.New().String()
	slug := e.generateSlug(item.Title)

	gallery, _ := json.Marshal(item.GalleryImages)
	attrs, _ := json.Marshal(item.Attributes)

	_, err := e.db.Exec(ctx, `
		INSERT INTO products (
			id, slug, title, description, short_description, price, regular_price, sale_price,
			ean, sku, external_id, image_url, gallery_images, category_id, category_path,
			brand, manufacturer, stock_status, stock_quantity, affiliate_url, button_text,
			delivery_time, attributes, feed_id, feed_checksum, is_active
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24, $25, true
		)
	`, id, slug, item.Title, item.Description, item.ShortDescription, item.Price,
		nullIfZero(item.RegularPrice), nullIfZero(item.SalePrice),
		nullIfEmpty(item.EAN), nullIfEmpty(item.SKU), nullIfEmpty(item.ExternalID),
		nullIfEmpty(item.ImageURL), gallery, categoryID, item.CategoryPath,
		nullIfEmpty(item.Brand), nullIfEmpty(item.Manufacturer),
		coalesce(item.StockStatus, "instock"), nullIfZero(float64(item.StockQuantity)),
		nullIfEmpty(item.AffiliateURL), coalesce(item.ButtonText, "Kúpiť"),
		nullIfEmpty(item.DeliveryTime), attrs, e.feed.ID, checksum)

	return err
}

func (e *ImportEngine) updateProduct(ctx context.Context, id string, item *models.FeedItem, categoryID *string, checksum string) error {
	gallery, _ := json.Marshal(item.GalleryImages)
	attrs, _ := json.Marshal(item.Attributes)

	_, err := e.db.Exec(ctx, `
		UPDATE products SET
			title = $2, description = $3, short_description = $4, price = $5,
			regular_price = $6, sale_price = $7, ean = $8, sku = $9, external_id = $10,
			image_url = $11, gallery_images = $12, category_id = $13, category_path = $14,
			brand = $15, manufacturer = $16, stock_status = $17, stock_quantity = $18,
			affiliate_url = $19, button_text = $20, delivery_time = $21, attributes = $22,
			feed_checksum = $23, updated_at = NOW()
		WHERE id = $1
	`, id, item.Title, item.Description, item.ShortDescription, item.Price,
		nullIfZero(item.RegularPrice), nullIfZero(item.SalePrice),
		nullIfEmpty(item.EAN), nullIfEmpty(item.SKU), nullIfEmpty(item.ExternalID),
		nullIfEmpty(item.ImageURL), gallery, categoryID, item.CategoryPath,
		nullIfEmpty(item.Brand), nullIfEmpty(item.Manufacturer),
		coalesce(item.StockStatus, "instock"), nullIfZero(float64(item.StockQuantity)),
		nullIfEmpty(item.AffiliateURL), coalesce(item.ButtonText, "Kúpiť"),
		nullIfEmpty(item.DeliveryTime), attrs, checksum)

	return err
}

func (e *ImportEngine) getOrCreateCategory(ctx context.Context, categoryPath string) *string {
	if categoryPath == "" {
		return nil
	}

	// Check cache
	if cachedID, ok := e.categoryCache[categoryPath]; ok {
		return &cachedID
	}

	// Parse path
	parts := strings.Split(categoryPath, "|")
	if len(parts) == 0 {
		parts = strings.Split(categoryPath, " > ")
	}
	if len(parts) == 0 {
		parts = strings.Split(categoryPath, "/")
	}

	var parentID *string
	var lastID string

	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}

		slug := e.generateSlug(name)

		// Find existing
		var categoryID string
		err := e.db.QueryRow(ctx, `
			SELECT id FROM categories 
			WHERE slug = $1 AND (parent_id = $2 OR (parent_id IS NULL AND $2 IS NULL))
			LIMIT 1
		`, slug, parentID).Scan(&categoryID)

		if err != nil {
			// Create
			categoryID = uuid.New().String()
			e.db.Exec(ctx, `
				INSERT INTO categories (id, name, slug, parent_id, product_count, is_active)
				VALUES ($1, $2, $3, $4, 0, true)
			`, categoryID, name, slug, parentID)
		}

		if categoryID != "" {
			lastID = categoryID
			parentID = &categoryID
		}
	}

	if lastID != "" {
		e.categoryCache[categoryPath] = lastID
		return &lastID
	}

	return nil
}

func (e *ImportEngine) generateSlug(text string) string {
	slug := strings.ToLower(text)

	replacements := map[string]string{
		"á": "a", "ä": "a", "č": "c", "ď": "d", "é": "e", "í": "i",
		"ĺ": "l", "ľ": "l", "ň": "n", "ó": "o", "ô": "o", "ŕ": "r",
		"š": "s", "ť": "t", "ú": "u", "ý": "y", "ž": "z",
	}
	for from, to := range replacements {
		slug = strings.ReplaceAll(slug, from, to)
	}

	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if len(slug) > 200 {
		slug = slug[:200]
	}

	// Add unique suffix to avoid collisions
	slug = slug + "-" + uuid.New().String()[:8]

	return slug
}

// Progress and status updates

func (e *ImportEngine) updateProgress(message string) {
	e.progress.Message = message
	e.progress.CurrentItem = message
	e.progress.Elapsed = int(time.Since(e.startTime).Seconds())
	e.progress.ETA = e.calculateETA()
	e.progress.Speed = e.calculateSpeed()

	if e.progress.Total > 0 {
		e.progress.Percent = (e.progress.Processed * 100) / e.progress.Total
	}
}

func (e *ImportEngine) updateProgressStats() {
	e.updateProgress(fmt.Sprintf("Processing... (%d/%d)", e.progress.Processed, e.progress.Total))
}

func (e *ImportEngine) calculateETA() int {
	if e.progress.Processed == 0 {
		return 0
	}
	elapsed := time.Since(e.startTime).Seconds()
	remaining := e.progress.Total - e.progress.Processed
	speed := float64(e.progress.Processed) / elapsed
	if speed > 0 {
		return int(float64(remaining) / speed)
	}
	return 0
}

func (e *ImportEngine) calculateSpeed() float64 {
	elapsed := time.Since(e.startTime).Seconds()
	if elapsed > 0 {
		return float64(e.progress.Processed) / elapsed
	}
	return 0
}

func (e *ImportEngine) log(level, message string) {
	entry := models.LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	}
	e.progress.Logs = append(e.progress.Logs, entry)

	if len(e.progress.Logs) > 100 {
		e.progress.Logs = e.progress.Logs[len(e.progress.Logs)-100:]
	}
}

func (e *ImportEngine) completeImport(ctx context.Context, history *models.ImportHistory) (*models.ImportHistory, error) {
	finishedAt := time.Now()
	duration := int(finishedAt.Sub(e.startTime).Seconds())

	history.FinishedAt = &finishedAt
	history.Duration = duration
	history.Processed = e.progress.Processed
	history.Created = e.progress.Created
	history.Updated = e.progress.Updated
	history.Skipped = e.progress.Skipped
	history.Errors = e.progress.Errors
	history.Status = models.ImportStatusCompleted

	e.saveHistory(ctx, history)
	e.updateFeedStatus(ctx, "active", "")
	e.updateCategoryCounts(ctx)

	e.progress.Status = models.ImportStatusCompleted
	e.progress.Message = "Import completed"
	e.updateProgress("Import completed")

	e.log("info", fmt.Sprintf(
		"Import completed: %d created, %d updated, %d skipped, %d errors. Duration: %ds",
		e.progress.Created, e.progress.Updated, e.progress.Skipped, e.progress.Errors, duration,
	))

	return history, nil
}

func (e *ImportEngine) failImport(ctx context.Context, history *models.ImportHistory, errorMsg string) (*models.ImportHistory, error) {
	finishedAt := time.Now()
	history.FinishedAt = &finishedAt
	history.Duration = int(finishedAt.Sub(e.startTime).Seconds())
	history.Status = models.ImportStatusFailed
	history.ErrorMessage = &errorMsg

	e.saveHistory(ctx, history)
	e.updateFeedStatus(ctx, "error", errorMsg)

	e.progress.Status = models.ImportStatusFailed
	e.progress.Message = errorMsg
	e.updateProgress(errorMsg)

	e.log("error", "Import failed: "+errorMsg)

	return history, fmt.Errorf(errorMsg)
}

func (e *ImportEngine) Stop() {
	e.mutex.Lock()
	e.shouldStop = true
	e.mutex.Unlock()
	e.log("info", "Stop requested")
}

func (e *ImportEngine) GetProgress() *models.ImportProgress {
	return e.progress
}

// Database helpers

func (e *ImportEngine) saveHistory(ctx context.Context, history *models.ImportHistory) error {
	_, err := e.db.Exec(ctx, `
		INSERT INTO import_history (
			id, feed_id, started_at, finished_at, duration,
			total_items, processed, created, updated, skipped, errors,
			status, error_message, triggered_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		ON CONFLICT (id) DO UPDATE SET
			finished_at = $4, duration = $5,
			total_items = $6, processed = $7, created = $8, updated = $9,
			skipped = $10, errors = $11, status = $12, error_message = $13
	`,
		history.ID, history.FeedID, history.StartedAt, history.FinishedAt, history.Duration,
		history.TotalItems, history.Processed, history.Created, history.Updated,
		history.Skipped, history.Errors, history.Status, history.ErrorMessage, history.TriggeredBy,
	)
	return err
}

func (e *ImportEngine) updateFeedStatus(ctx context.Context, status, errorMsg string) {
	e.db.Exec(ctx, `
		UPDATE feeds SET
			status = $2,
			last_run = NOW(),
			last_error = NULLIF($3, ''),
			total_products = (SELECT COUNT(*) FROM products WHERE feed_id = $1)
		WHERE id = $1
	`, e.feed.ID, status, errorMsg)
}

func (e *ImportEngine) updateCategoryCounts(ctx context.Context) {
	e.db.Exec(ctx, `
		UPDATE categories c SET product_count = (
			SELECT COUNT(*) FROM products p WHERE p.category_id = c.id AND p.is_active = true
		)
	`)
}

// Helper functions

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullIfZero(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}

func coalesce(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
