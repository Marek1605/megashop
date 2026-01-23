package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// USER
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type User struct {
	ID           string     `json:"id" db:"id"`
	Email        string     `json:"email" db:"email"`
	PasswordHash string     `json:"-" db:"password_hash"`
	Name         string     `json:"name" db:"name"`
	Role         string     `json:"role" db:"role"` // admin, editor
	IsActive     bool       `json:"is_active" db:"is_active"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	LastLogin    *time.Time `json:"last_login" db:"last_login"`
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// PRODUCT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type Product struct {
	ID               string    `json:"id" db:"id"`
	Slug             string    `json:"slug" db:"slug"`
	Title            string    `json:"title" db:"title"`
	Description      string    `json:"description" db:"description"`
	ShortDescription string    `json:"short_description" db:"short_description"`
	Price            float64   `json:"price" db:"price"`
	RegularPrice     *float64  `json:"regular_price" db:"regular_price"`
	SalePrice        *float64  `json:"sale_price" db:"sale_price"`
	Currency         string    `json:"currency" db:"currency"`
	EAN              *string   `json:"ean" db:"ean"`
	SKU              *string   `json:"sku" db:"sku"`
	MPN              *string   `json:"mpn" db:"mpn"`
	ExternalID       *string   `json:"external_id" db:"external_id"`
	ImageURL         *string   `json:"image_url" db:"image_url"`
	GalleryImages    JSONArray `json:"gallery_images" db:"gallery_images"`
	CategoryID       *string   `json:"category_id" db:"category_id"`
	CategoryPath     string    `json:"category_path" db:"category_path"`
	Brand            *string   `json:"brand" db:"brand"`
	Manufacturer     *string   `json:"manufacturer" db:"manufacturer"`
	StockStatus      string    `json:"stock_status" db:"stock_status"`
	StockQuantity    *int      `json:"stock_quantity" db:"stock_quantity"`
	IsActive         bool      `json:"is_active" db:"is_active"`
	IsFeatured       bool      `json:"is_featured" db:"is_featured"`
	Attributes       JSONMap   `json:"attributes" db:"attributes"`
	AffiliateURL     *string   `json:"affiliate_url" db:"affiliate_url"`
	ButtonText       *string   `json:"button_text" db:"button_text"`
	DeliveryTime     *string   `json:"delivery_time" db:"delivery_time"`
	FeedID           *string   `json:"feed_id" db:"feed_id"`
	FeedChecksum     *string   `json:"feed_checksum" db:"feed_checksum"`
	ViewCount        int       `json:"view_count" db:"view_count"`
	ClickCount       int       `json:"click_count" db:"click_count"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`

	// Joined fields
	Category *Category `json:"category,omitempty"`
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// CATEGORY
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type Category struct {
	ID           string      `json:"id" db:"id"`
	Name         string      `json:"name" db:"name"`
	Slug         string      `json:"slug" db:"slug"`
	Description  *string     `json:"description" db:"description"`
	ImageURL     *string     `json:"image_url" db:"image_url"`
	ParentID     *string     `json:"parent_id" db:"parent_id"`
	ProductCount int         `json:"product_count" db:"product_count"`
	SortOrder    int         `json:"sort_order" db:"sort_order"`
	IsActive     bool        `json:"is_active" db:"is_active"`
	CreatedAt    time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at" db:"updated_at"`
	Children     []*Category `json:"children,omitempty"`
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// FEED
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type FeedType string

const (
	FeedTypeXML  FeedType = "xml"
	FeedTypeCSV  FeedType = "csv"
	FeedTypeJSON FeedType = "json"
)

type FeedStatus string

const (
	FeedStatusActive  FeedStatus = "active"
	FeedStatusRunning FeedStatus = "running"
	FeedStatusError   FeedStatus = "error"
	FeedStatusPaused  FeedStatus = "paused"
)

type ImportMode string

const (
	ImportModeCreateUpdate ImportMode = "create_update"
	ImportModeCreateOnly   ImportMode = "create_only"
	ImportModeUpdateOnly   ImportMode = "update_only"
)

type MatchBy string

const (
	MatchByEAN        MatchBy = "ean"
	MatchBySKU        MatchBy = "sku"
	MatchByTitle      MatchBy = "title"
	MatchByExternalID MatchBy = "external_id"
)

type Feed struct {
	ID              string     `json:"id" db:"id"`
	Name            string     `json:"name" db:"name"`
	Description     string     `json:"description" db:"description"`
	FeedURL         string     `json:"feed_url" db:"feed_url"`
	FeedType        FeedType   `json:"feed_type" db:"feed_type"`
	XMLItemPath     string     `json:"xml_item_path" db:"xml_item_path"`
	CSVDelimiter    string     `json:"csv_delimiter" db:"csv_delimiter"`
	CSVHasHeader    bool       `json:"csv_has_header" db:"csv_has_header"`
	ImportMode      ImportMode `json:"import_mode" db:"import_mode"`
	MatchBy         MatchBy    `json:"match_by" db:"match_by"`
	DefaultCategory *string    `json:"default_category" db:"default_category"`
	ImportImages    bool       `json:"import_images" db:"import_images"`
	CreateAttributes bool      `json:"create_attributes" db:"create_attributes"`
	ScheduleEnabled bool       `json:"schedule_enabled" db:"schedule_enabled"`
	ScheduleCron    string     `json:"schedule_cron" db:"schedule_cron"`
	Active          bool       `json:"active" db:"active"`
	Status          FeedStatus `json:"status" db:"status"`
	LastRun         *time.Time `json:"last_run" db:"last_run"`
	LastError       *string    `json:"last_error" db:"last_error"`
	TotalProducts   int        `json:"total_products" db:"total_products"`
	FieldMappings   JSONArray  `json:"field_mappings" db:"field_mappings"`
	Settings        JSONMap    `json:"settings" db:"settings"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

type FieldMapping struct {
	ID             string `json:"id"`
	SourceField    string `json:"source_field"`
	TargetField    string `json:"target_field"`
	TransformType  string `json:"transform_type"`
	TransformValue string `json:"transform_value"`
	DefaultValue   string `json:"default_value"`
	IsRequired     bool   `json:"is_required"`
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// IMPORT
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type ImportStatus string

const (
	ImportStatusIdle      ImportStatus = "idle"
	ImportStatusRunning   ImportStatus = "running"
	ImportStatusCompleted ImportStatus = "completed"
	ImportStatusFailed    ImportStatus = "failed"
	ImportStatusCancelled ImportStatus = "cancelled"
)

type ImportHistory struct {
	ID           string       `json:"id" db:"id"`
	FeedID       string       `json:"feed_id" db:"feed_id"`
	StartedAt    time.Time    `json:"started_at" db:"started_at"`
	FinishedAt   *time.Time   `json:"finished_at" db:"finished_at"`
	Duration     int          `json:"duration" db:"duration"`
	TotalItems   int          `json:"total_items" db:"total_items"`
	Processed    int          `json:"processed" db:"processed"`
	Created      int          `json:"created" db:"created"`
	Updated      int          `json:"updated" db:"updated"`
	Skipped      int          `json:"skipped" db:"skipped"`
	Errors       int          `json:"errors" db:"errors"`
	Status       ImportStatus `json:"status" db:"status"`
	ErrorMessage *string      `json:"error_message" db:"error_message"`
	TriggeredBy  string       `json:"triggered_by" db:"triggered_by"`
}

type ImportProgress struct {
	FeedID      string       `json:"feed_id"`
	HistoryID   string       `json:"history_id"`
	Status      ImportStatus `json:"status"`
	Percent     int          `json:"percent"`
	Total       int          `json:"total"`
	Processed   int          `json:"processed"`
	Created     int          `json:"created"`
	Updated     int          `json:"updated"`
	Skipped     int          `json:"skipped"`
	Errors      int          `json:"errors"`
	Message     string       `json:"message"`
	CurrentItem string       `json:"current_item"`
	Elapsed     int          `json:"elapsed"`
	ETA         int          `json:"eta"`
	Speed       float64      `json:"speed"`
	Logs        []LogEntry   `json:"logs"`
}

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// FeedItem - Item mapovaný z feedu
type FeedItem struct {
	Title            string
	Description      string
	ShortDescription string
	Price            float64
	RegularPrice     float64
	SalePrice        float64
	EAN              string
	SKU              string
	MPN              string
	ExternalID       string
	ImageURL         string
	GalleryImages    []string
	CategoryPath     string
	Brand            string
	Manufacturer     string
	StockStatus      string
	StockQuantity    int
	Attributes       map[string]string
	AffiliateURL     string
	ButtonText       string
	DeliveryTime     string
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// SHOP CONFIG
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type ShopConfig struct {
	ID              string    `json:"id" db:"id"`
	ShopName        string    `json:"shop_name" db:"shop_name"`
	ShopURL         string    `json:"shop_url" db:"shop_url"`
	Logo            *string   `json:"logo" db:"logo"`
	Favicon         *string   `json:"favicon" db:"favicon"`
	Currency        string    `json:"currency" db:"currency"`
	Locale          string    `json:"locale" db:"locale"`
	Template        string    `json:"template" db:"template"` // aurora, minimal, modern
	PrimaryColor    string    `json:"primary_color" db:"primary_color"`
	SecondaryColor  string    `json:"secondary_color" db:"secondary_color"`
	GoogleAnalytics *string   `json:"google_analytics" db:"google_analytics"`
	MetaTitle       *string   `json:"meta_title" db:"meta_title"`
	MetaDescription *string   `json:"meta_description" db:"meta_description"`
	CustomCSS       *string   `json:"custom_css" db:"custom_css"`
	CustomJS        *string   `json:"custom_js" db:"custom_js"`
	Settings        JSONMap   `json:"settings" db:"settings"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// JSON HELPERS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type JSONMap map[string]interface{}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

type JSONArray []interface{}

func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = []interface{}{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// TARGET FIELDS
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

type TargetFieldInfo struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Group    string `json:"group"`
	Required bool   `json:"required"`
}

var TargetFields = []TargetFieldInfo{
	{Key: "ean", Label: "EAN / GTIN", Group: "identifiers", Required: false},
	{Key: "sku", Label: "SKU", Group: "identifiers", Required: false},
	{Key: "external_id", Label: "External ID", Group: "identifiers", Required: false},
	{Key: "title", Label: "Product Name", Group: "basic", Required: true},
	{Key: "description", Label: "Description", Group: "basic", Required: false},
	{Key: "short_description", Label: "Short Description", Group: "basic", Required: false},
	{Key: "price", Label: "Price", Group: "pricing", Required: true},
	{Key: "regular_price", Label: "Regular Price", Group: "pricing", Required: false},
	{Key: "sale_price", Label: "Sale Price", Group: "pricing", Required: false},
	{Key: "stock_status", Label: "Stock Status", Group: "inventory", Required: false},
	{Key: "stock_quantity", Label: "Stock Quantity", Group: "inventory", Required: false},
	{Key: "image_url", Label: "Main Image", Group: "media", Required: false},
	{Key: "gallery_images", Label: "Gallery Images", Group: "media", Required: false},
	{Key: "category", Label: "Category", Group: "taxonomy", Required: false},
	{Key: "brand", Label: "Brand", Group: "attributes", Required: false},
	{Key: "manufacturer", Label: "Manufacturer", Group: "attributes", Required: false},
	{Key: "attributes", Label: "Attributes (PARAM)", Group: "attributes", Required: false},
	{Key: "affiliate_url", Label: "Affiliate URL", Group: "affiliate", Required: false},
	{Key: "button_text", Label: "Button Text", Group: "affiliate", Required: false},
	{Key: "delivery_time", Label: "Delivery Time", Group: "other", Required: false},
}
