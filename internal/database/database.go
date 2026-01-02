package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func Connect() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return err
	}

	// Connection pool settings for high performance
	config.MaxConns = 100
	config.MinConns = 10
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	Pool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return err
	}

	// Test connection
	if err := Pool.Ping(ctx); err != nil {
		return err
	}

	fmt.Println("✅ Database connected")
	return nil
}

func Close() {
	if Pool != nil {
		Pool.Close()
	}
}

func Migrate() error {
	ctx := context.Background()

	migrations := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			name VARCHAR(255),
			plan VARCHAR(50) DEFAULT 'FREE',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,

		// Shops table
		`CREATE TABLE IF NOT EXISTS shops (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) UNIQUE NOT NULL,
			description TEXT,
			logo VARCHAR(500),
			currency VARCHAR(10) DEFAULT 'EUR',
			language VARCHAR(10) DEFAULT 'sk',
			primary_color VARCHAR(20) DEFAULT '#6366f1',
			email VARCHAR(255),
			phone VARCHAR(50),
			address TEXT,
			facebook VARCHAR(500),
			instagram VARCHAR(500),
			meta_title VARCHAR(255),
			meta_description TEXT,
			is_active BOOLEAN DEFAULT true,
			is_published BOOLEAN DEFAULT false,
			custom_domain VARCHAR(255),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_shops_user_id ON shops(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_shops_slug ON shops(slug)`,

		// Categories table
		`CREATE TABLE IF NOT EXISTS categories (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			parent_id UUID REFERENCES categories(id) ON DELETE SET NULL,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) NOT NULL,
			description TEXT,
			image VARCHAR(500),
			position INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(shop_id, slug)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_categories_shop_id ON categories(shop_id)`,

		// Products table
		`CREATE TABLE IF NOT EXISTS products (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) NOT NULL,
			description TEXT,
			short_description VARCHAR(500),
			price DECIMAL(10,2) NOT NULL,
			compare_price DECIMAL(10,2),
			cost_price DECIMAL(10,2),
			sku VARCHAR(100),
			barcode VARCHAR(100),
			quantity INT DEFAULT 0,
			track_inventory BOOLEAN DEFAULT true,
			allow_backorder BOOLEAN DEFAULT false,
			weight DECIMAL(10,3),
			width DECIMAL(10,2),
			height DECIMAL(10,2),
			length DECIMAL(10,2),
			meta_title VARCHAR(255),
			meta_description TEXT,
			is_active BOOLEAN DEFAULT true,
			is_featured BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(shop_id, slug)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_products_shop_id ON products(shop_id)`,
		`CREATE INDEX IF NOT EXISTS idx_products_category_id ON products(category_id)`,
		`CREATE INDEX IF NOT EXISTS idx_products_sku ON products(sku)`,
		`CREATE INDEX IF NOT EXISTS idx_products_is_active ON products(is_active)`,

		// Product images table
		`CREATE TABLE IF NOT EXISTS product_images (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
			url VARCHAR(500) NOT NULL,
			alt VARCHAR(255),
			position INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_product_images_product_id ON product_images(product_id)`,

		// Product variants table
		`CREATE TABLE IF NOT EXISTS product_variants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			sku VARCHAR(100),
			price DECIMAL(10,2) NOT NULL,
			quantity INT DEFAULT 0,
			options JSONB,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_product_variants_product_id ON product_variants(product_id)`,

		// Customers table
		`CREATE TABLE IF NOT EXISTS customers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			email VARCHAR(255) NOT NULL,
			first_name VARCHAR(255),
			last_name VARCHAR(255),
			phone VARCHAR(50),
			address TEXT,
			city VARCHAR(255),
			zip VARCHAR(20),
			country VARCHAR(10) DEFAULT 'SK',
			accepts_marketing BOOLEAN DEFAULT false,
			notes TEXT,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(shop_id, email)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_shop_id ON customers(shop_id)`,

		// Orders table
		`CREATE TABLE IF NOT EXISTS orders (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			customer_id UUID REFERENCES customers(id),
			order_number VARCHAR(50) UNIQUE NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			payment_status VARCHAR(50) DEFAULT 'pending',
			subtotal DECIMAL(10,2) NOT NULL,
			shipping DECIMAL(10,2) DEFAULT 0,
			tax DECIMAL(10,2) DEFAULT 0,
			discount DECIMAL(10,2) DEFAULT 0,
			total DECIMAL(10,2) NOT NULL,
			currency VARCHAR(10) DEFAULT 'EUR',
			shipping_first_name VARCHAR(255),
			shipping_last_name VARCHAR(255),
			shipping_company VARCHAR(255),
			shipping_address TEXT,
			shipping_city VARCHAR(255),
			shipping_zip VARCHAR(20),
			shipping_country VARCHAR(10),
			shipping_phone VARCHAR(50),
			billing_first_name VARCHAR(255),
			billing_last_name VARCHAR(255),
			billing_company VARCHAR(255),
			billing_address TEXT,
			billing_city VARCHAR(255),
			billing_zip VARCHAR(20),
			billing_country VARCHAR(10),
			billing_phone VARCHAR(50),
			billing_email VARCHAR(255),
			payment_method VARCHAR(100),
			payment_id VARCHAR(255),
			shipping_method VARCHAR(100),
			tracking_number VARCHAR(255),
			customer_note TEXT,
			internal_note TEXT,
			coupon_code VARCHAR(100),
			coupon_id UUID,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_shop_id ON orders(shop_id)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_order_number ON orders(order_number)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at)`,

		// Order items table
		`CREATE TABLE IF NOT EXISTS order_items (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			product_id UUID REFERENCES products(id),
			variant_id UUID REFERENCES product_variants(id),
			name VARCHAR(255) NOT NULL,
			sku VARCHAR(100),
			quantity INT NOT NULL,
			price DECIMAL(10,2) NOT NULL,
			total DECIMAL(10,2) NOT NULL,
			variant_name VARCHAR(255),
			variant_options JSONB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id)`,

		// Order history table
		`CREATE TABLE IF NOT EXISTS order_history (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			status VARCHAR(50) NOT NULL,
			note TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_order_history_order_id ON order_history(order_id)`,

		// Shipping methods table
		`CREATE TABLE IF NOT EXISTS shipping_methods (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			price DECIMAL(10,2) NOT NULL,
			free_from DECIMAL(10,2),
			estimated_days VARCHAR(50),
			carrier VARCHAR(100),
			countries TEXT[] DEFAULT ARRAY['SK', 'CZ'],
			max_weight DECIMAL(10,3),
			is_active BOOLEAN DEFAULT true,
			position INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_shipping_methods_shop_id ON shipping_methods(shop_id)`,

		// Payment methods table
		`CREATE TABLE IF NOT EXISTS payment_methods (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			type VARCHAR(50) NOT NULL,
			fee DECIMAL(10,2) DEFAULT 0,
			instructions TEXT,
			is_active BOOLEAN DEFAULT true,
			position INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_methods_shop_id ON payment_methods(shop_id)`,

		// Coupons table
		`CREATE TABLE IF NOT EXISTS coupons (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			code VARCHAR(100) NOT NULL,
			description TEXT,
			type VARCHAR(50) DEFAULT 'percentage',
			value DECIMAL(10,2) NOT NULL,
			min_order_value DECIMAL(10,2),
			max_uses INT,
			used_count INT DEFAULT 0,
			starts_at TIMESTAMP,
			expires_at TIMESTAMP,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(shop_id, code)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_coupons_shop_id ON coupons(shop_id)`,

		// Payments table
		`CREATE TABLE IF NOT EXISTS payments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			gateway VARCHAR(50) NOT NULL,
			gateway_id VARCHAR(255) NOT NULL,
			order_id UUID NOT NULL,
			order_number VARCHAR(50) NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			currency VARCHAR(10) DEFAULT 'EUR',
			status VARCHAR(50) DEFAULT 'pending',
			metadata JSONB,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_payments_gateway_id ON payments(gateway_id)`,
		`CREATE INDEX IF NOT EXISTS idx_payments_order_id ON payments(order_id)`,

		// Shop settings table
		`CREATE TABLE IF NOT EXISTS shop_settings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID UNIQUE NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			company_name VARCHAR(255),
			ico VARCHAR(50),
			dic VARCHAR(50),
			ic_dph VARCHAR(50),
			bank_name VARCHAR(255),
			iban VARCHAR(50),
			swift VARCHAR(20),
			invoice_prefix VARCHAR(20) DEFAULT 'FA',
			invoice_next_number INT DEFAULT 1,
			invoice_footer TEXT,
			tax_rate DECIMAL(5,2) DEFAULT 20,
			prices_include_tax BOOLEAN DEFAULT true,
			min_order_value DECIMAL(10,2),
			order_notify_email VARCHAR(255),
			low_stock_threshold INT DEFAULT 5,
			terms_url VARCHAR(500),
			privacy_url VARCHAR(500),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,

		// Invoices table
		`CREATE TABLE IF NOT EXISTS invoices (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			invoice_number VARCHAR(50) UNIQUE NOT NULL,
			type VARCHAR(50) DEFAULT 'invoice',
			issue_date TIMESTAMP DEFAULT NOW(),
			due_date TIMESTAMP NOT NULL,
			paid_at TIMESTAMP,
			status VARCHAR(50) DEFAULT 'unpaid',
			subtotal DECIMAL(10,2) NOT NULL,
			tax DECIMAL(10,2) DEFAULT 0,
			total DECIMAL(10,2) NOT NULL,
			currency VARCHAR(10) DEFAULT 'EUR',
			supplier_name VARCHAR(255),
			supplier_address TEXT,
			supplier_city VARCHAR(255),
			supplier_zip VARCHAR(20),
			supplier_country VARCHAR(10),
			supplier_ico VARCHAR(50),
			supplier_dic VARCHAR(50),
			supplier_ic_dph VARCHAR(50),
			supplier_iban VARCHAR(50),
			customer_name VARCHAR(255),
			customer_address TEXT,
			customer_city VARCHAR(255),
			customer_zip VARCHAR(20),
			customer_country VARCHAR(10),
			customer_ico VARCHAR(50),
			customer_dic VARCHAR(50),
			customer_ic_dph VARCHAR(50),
			customer_email VARCHAR(255),
			items JSONB NOT NULL,
			note TEXT,
			order_id UUID,
			order_number VARCHAR(50),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_shop_id ON invoices(shop_id)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_order_id ON invoices(order_id)`,

		// Reviews table
		`CREATE TABLE IF NOT EXISTS reviews (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
			customer_id UUID,
			rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
			title VARCHAR(255),
			content TEXT,
			author_name VARCHAR(255) NOT NULL,
			author_email VARCHAR(255),
			is_approved BOOLEAN DEFAULT false,
			is_verified BOOLEAN DEFAULT false,
			response TEXT,
			responded_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_reviews_product_id ON reviews(product_id)`,

		// Page views / Analytics
		`CREATE TABLE IF NOT EXISTS page_views (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			page VARCHAR(500),
			referrer VARCHAR(500),
			user_agent TEXT,
			ip VARCHAR(50),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_page_views_shop_id ON page_views(shop_id)`,
		`CREATE INDEX IF NOT EXISTS idx_page_views_created_at ON page_views(created_at)`,

		// Daily stats (pre-aggregated for speed)
		`CREATE TABLE IF NOT EXISTS daily_stats (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
			date DATE NOT NULL,
			page_views INT DEFAULT 0,
			unique_visitors INT DEFAULT 0,
			orders INT DEFAULT 0,
			revenue DECIMAL(10,2) DEFAULT 0,
			UNIQUE(shop_id, date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_stats_shop_id_date ON daily_stats(shop_id, date)`,
	}

	for _, sql := range migrations {
		if _, err := Pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migration failed: %s - %w", sql[:50], err)
		}
	}

	fmt.Println("✅ Migrations completed")
	return nil
}
