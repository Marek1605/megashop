package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// User model
type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Name         *string    `json:"name"`
	Plan         string     `json:"plan"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Shop model
type Shop struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	Name            string     `json:"name"`
	Slug            string     `json:"slug"`
	Description     *string    `json:"description"`
	Logo            *string    `json:"logo"`
	Currency        string     `json:"currency"`
	Language        string     `json:"language"`
	PrimaryColor    string     `json:"primary_color"`
	Email           *string    `json:"email"`
	Phone           *string    `json:"phone"`
	Address         *string    `json:"address"`
	Facebook        *string    `json:"facebook"`
	Instagram       *string    `json:"instagram"`
	MetaTitle       *string    `json:"meta_title"`
	MetaDescription *string    `json:"meta_description"`
	IsActive        bool       `json:"is_active"`
	IsPublished     bool       `json:"is_published"`
	CustomDomain    *string    `json:"custom_domain"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Category model
type Category struct {
	ID          uuid.UUID  `json:"id"`
	ShopID      uuid.UUID  `json:"shop_id"`
	ParentID    *uuid.UUID `json:"parent_id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description *string    `json:"description"`
	Image       *string    `json:"image"`
	Position    int        `json:"position"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	// Relations
	Products    []Product  `json:"products,omitempty"`
	ProductCount int       `json:"product_count,omitempty"`
}

// Product model
type Product struct {
	ID               uuid.UUID        `json:"id"`
	ShopID           uuid.UUID        `json:"shop_id"`
	CategoryID       *uuid.UUID       `json:"category_id"`
	Name             string           `json:"name"`
	Slug             string           `json:"slug"`
	Description      *string          `json:"description"`
	ShortDescription *string          `json:"short_description"`
	Price            float64          `json:"price"`
	ComparePrice     *float64         `json:"compare_price"`
	CostPrice        *float64         `json:"cost_price"`
	SKU              *string          `json:"sku"`
	Barcode          *string          `json:"barcode"`
	Quantity         int              `json:"quantity"`
	TrackInventory   bool             `json:"track_inventory"`
	AllowBackorder   bool             `json:"allow_backorder"`
	Weight           *float64         `json:"weight"`
	Width            *float64         `json:"width"`
	Height           *float64         `json:"height"`
	Length           *float64         `json:"length"`
	MetaTitle        *string          `json:"meta_title"`
	MetaDescription  *string          `json:"meta_description"`
	IsActive         bool             `json:"is_active"`
	IsFeatured       bool             `json:"is_featured"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	// Relations
	Images           []ProductImage   `json:"images,omitempty"`
	Variants         []ProductVariant `json:"variants,omitempty"`
	Category         *Category        `json:"category,omitempty"`
}

// ProductImage model
type ProductImage struct {
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"product_id"`
	URL       string    `json:"url"`
	Alt       *string   `json:"alt"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// ProductVariant model
type ProductVariant struct {
	ID        uuid.UUID       `json:"id"`
	ProductID uuid.UUID       `json:"product_id"`
	Name      string          `json:"name"`
	SKU       *string         `json:"sku"`
	Price     float64         `json:"price"`
	Quantity  int             `json:"quantity"`
	Options   json.RawMessage `json:"options"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Customer model
type Customer struct {
	ID               uuid.UUID  `json:"id"`
	ShopID           uuid.UUID  `json:"shop_id"`
	Email            string     `json:"email"`
	FirstName        *string    `json:"first_name"`
	LastName         *string    `json:"last_name"`
	Phone            *string    `json:"phone"`
	Address          *string    `json:"address"`
	City             *string    `json:"city"`
	Zip              *string    `json:"zip"`
	Country          string     `json:"country"`
	AcceptsMarketing bool       `json:"accepts_marketing"`
	Notes            *string    `json:"notes"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	// Computed
	OrdersCount      int        `json:"orders_count,omitempty"`
	TotalSpent       float64    `json:"total_spent,omitempty"`
}

// Order model
type Order struct {
	ID                uuid.UUID    `json:"id"`
	ShopID            uuid.UUID    `json:"shop_id"`
	CustomerID        *uuid.UUID   `json:"customer_id"`
	OrderNumber       string       `json:"order_number"`
	Status            string       `json:"status"`
	PaymentStatus     string       `json:"payment_status"`
	Subtotal          float64      `json:"subtotal"`
	Shipping          float64      `json:"shipping"`
	Tax               float64      `json:"tax"`
	Discount          float64      `json:"discount"`
	Total             float64      `json:"total"`
	Currency          string       `json:"currency"`
	ShippingFirstName *string      `json:"shipping_first_name"`
	ShippingLastName  *string      `json:"shipping_last_name"`
	ShippingCompany   *string      `json:"shipping_company"`
	ShippingAddress   *string      `json:"shipping_address"`
	ShippingCity      *string      `json:"shipping_city"`
	ShippingZip       *string      `json:"shipping_zip"`
	ShippingCountry   *string      `json:"shipping_country"`
	ShippingPhone     *string      `json:"shipping_phone"`
	BillingFirstName  *string      `json:"billing_first_name"`
	BillingLastName   *string      `json:"billing_last_name"`
	BillingCompany    *string      `json:"billing_company"`
	BillingAddress    *string      `json:"billing_address"`
	BillingCity       *string      `json:"billing_city"`
	BillingZip        *string      `json:"billing_zip"`
	BillingCountry    *string      `json:"billing_country"`
	BillingPhone      *string      `json:"billing_phone"`
	BillingEmail      *string      `json:"billing_email"`
	PaymentMethod     *string      `json:"payment_method"`
	PaymentID         *string      `json:"payment_id"`
	ShippingMethod    *string      `json:"shipping_method"`
	TrackingNumber    *string      `json:"tracking_number"`
	CustomerNote      *string      `json:"customer_note"`
	InternalNote      *string      `json:"internal_note"`
	CouponCode        *string      `json:"coupon_code"`
	CouponID          *uuid.UUID   `json:"coupon_id"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	// Relations
	Items             []OrderItem    `json:"items,omitempty"`
	History           []OrderHistory `json:"history,omitempty"`
	Customer          *Customer      `json:"customer,omitempty"`
}

// OrderItem model
type OrderItem struct {
	ID             uuid.UUID       `json:"id"`
	OrderID        uuid.UUID       `json:"order_id"`
	ProductID      *uuid.UUID      `json:"product_id"`
	VariantID      *uuid.UUID      `json:"variant_id"`
	Name           string          `json:"name"`
	SKU            *string         `json:"sku"`
	Quantity       int             `json:"quantity"`
	Price          float64         `json:"price"`
	Total          float64         `json:"total"`
	VariantName    *string         `json:"variant_name"`
	VariantOptions json.RawMessage `json:"variant_options"`
}

// OrderHistory model
type OrderHistory struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	Status    string    `json:"status"`
	Note      *string   `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// ShippingMethod model
type ShippingMethod struct {
	ID            uuid.UUID  `json:"id"`
	ShopID        uuid.UUID  `json:"shop_id"`
	Name          string     `json:"name"`
	Description   *string    `json:"description"`
	Price         float64    `json:"price"`
	FreeFrom      *float64   `json:"free_from"`
	EstimatedDays *string    `json:"estimated_days"`
	Carrier       *string    `json:"carrier"`
	Countries     []string   `json:"countries"`
	MaxWeight     *float64   `json:"max_weight"`
	IsActive      bool       `json:"is_active"`
	Position      int        `json:"position"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// PaymentMethod model
type PaymentMethod struct {
	ID           uuid.UUID `json:"id"`
	ShopID       uuid.UUID `json:"shop_id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	Type         string    `json:"type"`
	Fee          float64   `json:"fee"`
	Instructions *string   `json:"instructions"`
	IsActive     bool      `json:"is_active"`
	Position     int       `json:"position"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Coupon model
type Coupon struct {
	ID            uuid.UUID  `json:"id"`
	ShopID        uuid.UUID  `json:"shop_id"`
	Code          string     `json:"code"`
	Description   *string    `json:"description"`
	Type          string     `json:"type"`
	Value         float64    `json:"value"`
	MinOrderValue *float64   `json:"min_order_value"`
	MaxUses       *int       `json:"max_uses"`
	UsedCount     int        `json:"used_count"`
	StartsAt      *time.Time `json:"starts_at"`
	ExpiresAt     *time.Time `json:"expires_at"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Payment model
type Payment struct {
	ID          uuid.UUID       `json:"id"`
	Gateway     string          `json:"gateway"`
	GatewayID   string          `json:"gateway_id"`
	OrderID     uuid.UUID       `json:"order_id"`
	OrderNumber string          `json:"order_number"`
	Amount      float64         `json:"amount"`
	Currency    string          `json:"currency"`
	Status      string          `json:"status"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ShopSettings model
type ShopSettings struct {
	ID                uuid.UUID `json:"id"`
	ShopID            uuid.UUID `json:"shop_id"`
	CompanyName       *string   `json:"company_name"`
	ICO               *string   `json:"ico"`
	DIC               *string   `json:"dic"`
	ICDPH             *string   `json:"ic_dph"`
	BankName          *string   `json:"bank_name"`
	IBAN              *string   `json:"iban"`
	SWIFT             *string   `json:"swift"`
	InvoicePrefix     string    `json:"invoice_prefix"`
	InvoiceNextNumber int       `json:"invoice_next_number"`
	InvoiceFooter     *string   `json:"invoice_footer"`
	TaxRate           float64   `json:"tax_rate"`
	PricesIncludeTax  bool      `json:"prices_include_tax"`
	MinOrderValue     *float64  `json:"min_order_value"`
	OrderNotifyEmail  *string   `json:"order_notify_email"`
	LowStockThreshold int       `json:"low_stock_threshold"`
	TermsURL          *string   `json:"terms_url"`
	PrivacyURL        *string   `json:"privacy_url"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Invoice model
type Invoice struct {
	ID               uuid.UUID       `json:"id"`
	ShopID           uuid.UUID       `json:"shop_id"`
	InvoiceNumber    string          `json:"invoice_number"`
	Type             string          `json:"type"`
	IssueDate        time.Time       `json:"issue_date"`
	DueDate          time.Time       `json:"due_date"`
	PaidAt           *time.Time      `json:"paid_at"`
	Status           string          `json:"status"`
	Subtotal         float64         `json:"subtotal"`
	Tax              float64         `json:"tax"`
	Total            float64         `json:"total"`
	Currency         string          `json:"currency"`
	SupplierName     *string         `json:"supplier_name"`
	SupplierAddress  *string         `json:"supplier_address"`
	SupplierCity     *string         `json:"supplier_city"`
	SupplierZip      *string         `json:"supplier_zip"`
	SupplierCountry  *string         `json:"supplier_country"`
	SupplierICO      *string         `json:"supplier_ico"`
	SupplierDIC      *string         `json:"supplier_dic"`
	SupplierICDPH    *string         `json:"supplier_ic_dph"`
	SupplierIBAN     *string         `json:"supplier_iban"`
	CustomerName     *string         `json:"customer_name"`
	CustomerAddress  *string         `json:"customer_address"`
	CustomerCity     *string         `json:"customer_city"`
	CustomerZip      *string         `json:"customer_zip"`
	CustomerCountry  *string         `json:"customer_country"`
	CustomerICO      *string         `json:"customer_ico"`
	CustomerDIC      *string         `json:"customer_dic"`
	CustomerICDPH    *string         `json:"customer_ic_dph"`
	CustomerEmail    *string         `json:"customer_email"`
	Items            json.RawMessage `json:"items"`
	Note             *string         `json:"note"`
	OrderID          *uuid.UUID      `json:"order_id"`
	OrderNumber      *string         `json:"order_number"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// DailyStats model
type DailyStats struct {
	ID             uuid.UUID `json:"id"`
	ShopID         uuid.UUID `json:"shop_id"`
	Date           time.Time `json:"date"`
	PageViews      int       `json:"page_views"`
	UniqueVisitors int       `json:"unique_visitors"`
	Orders         int       `json:"orders"`
	Revenue        float64   `json:"revenue"`
}

// ========================================
// Request/Response DTOs
// ========================================

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type AuthResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type CreateShopRequest struct {
	Name        string  `json:"name" validate:"required,min=2"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	Currency    string  `json:"currency"`
	Language    string  `json:"language"`
}

type CreateProductRequest struct {
	Name             string                `json:"name" validate:"required"`
	Slug             string                `json:"slug"`
	Description      *string               `json:"description"`
	ShortDescription *string               `json:"short_description"`
	Price            float64               `json:"price" validate:"required,gte=0"`
	ComparePrice     *float64              `json:"compare_price"`
	CostPrice        *float64              `json:"cost_price"`
	SKU              *string               `json:"sku"`
	Barcode          *string               `json:"barcode"`
	Quantity         int                   `json:"quantity"`
	CategoryID       *uuid.UUID            `json:"category_id"`
	IsActive         *bool                 `json:"is_active"`
	IsFeatured       *bool                 `json:"is_featured"`
	Images           []ProductImageRequest `json:"images"`
	MetaTitle        *string               `json:"meta_title"`
	MetaDescription  *string               `json:"meta_description"`
}

type ProductImageRequest struct {
	URL string  `json:"url" validate:"required,url"`
	Alt *string `json:"alt"`
}

type CreateOrderRequest struct {
	Items          []OrderItemRequest `json:"items" validate:"required,min=1"`
	Shipping       AddressRequest     `json:"shipping"`
	Billing        AddressRequest     `json:"billing"`
	ShippingMethod string             `json:"shipping_method"`
	PaymentMethod  string             `json:"payment_method"`
	CustomerNote   *string            `json:"customer_note"`
	CouponCode     *string            `json:"coupon_code"`
}

type OrderItemRequest struct {
	ProductID uuid.UUID  `json:"product_id" validate:"required"`
	VariantID *uuid.UUID `json:"variant_id"`
	Quantity  int        `json:"quantity" validate:"required,gte=1"`
}

type AddressRequest struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Company   *string `json:"company"`
	Address   string  `json:"address"`
	City      string  `json:"city"`
	Zip       string  `json:"zip"`
	Country   string  `json:"country"`
	Phone     *string `json:"phone"`
	Email     string  `json:"email"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
}

type AnalyticsResponse struct {
	Period     int                    `json:"period"`
	Summary    AnalyticsSummary       `json:"summary"`
	DailyStats []DailyStats           `json:"daily_stats"`
	TopProducts []ProductAnalytics    `json:"top_products"`
}

type AnalyticsSummary struct {
	TotalRevenue      float64 `json:"total_revenue"`
	TotalOrders       int     `json:"total_orders"`
	CompletedOrders   int     `json:"completed_orders"`
	PendingOrders     int     `json:"pending_orders"`
	CancelledOrders   int     `json:"cancelled_orders"`
	AverageOrderValue float64 `json:"average_order_value"`
	NewCustomers      int     `json:"new_customers"`
	TotalCustomers    int     `json:"total_customers"`
	RevenueChange     float64 `json:"revenue_change"`
	OrdersChange      float64 `json:"orders_change"`
}

type ProductAnalytics struct {
	ProductID uuid.UUID `json:"product_id"`
	Name      string    `json:"name"`
	Quantity  int       `json:"quantity"`
	Revenue   float64   `json:"revenue"`
}
