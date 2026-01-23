-- EshopBuilder v3 - Database Schema
-- ================================

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
-- USERS
-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'admin',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login TIMESTAMP WITH TIME ZONE
);

-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
-- CATEGORIES
-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    image_url TEXT,
    parent_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    product_count INTEGER DEFAULT 0,
    sort_order INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories(parent_id);
CREATE INDEX IF NOT EXISTS idx_categories_slug ON categories(slug);

-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
-- FEEDS
-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

CREATE TABLE IF NOT EXISTS feeds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    feed_url TEXT NOT NULL,
    feed_type VARCHAR(20) NOT NULL DEFAULT 'xml', -- xml, csv, json
    xml_item_path VARCHAR(100) DEFAULT 'SHOPITEM',
    csv_delimiter VARCHAR(5) DEFAULT ';',
    csv_has_header BOOLEAN DEFAULT true,
    import_mode VARCHAR(20) DEFAULT 'create_update', -- create_update, create_only, update_only
    match_by VARCHAR(20) DEFAULT 'ean', -- ean, sku, title, external_id
    default_category UUID REFERENCES categories(id),
    import_images BOOLEAN DEFAULT true,
    create_attributes BOOLEAN DEFAULT true,
    schedule_enabled BOOLEAN DEFAULT false,
    schedule_cron VARCHAR(100),
    active BOOLEAN DEFAULT true,
    status VARCHAR(20) DEFAULT 'active', -- active, running, error, paused
    last_run TIMESTAMP WITH TIME ZONE,
    last_error TEXT,
    total_products INTEGER DEFAULT 0,
    field_mappings JSONB DEFAULT '[]'::jsonb,
    settings JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
-- PRODUCTS
-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(255) UNIQUE NOT NULL,
    title VARCHAR(500) NOT NULL,
    description TEXT,
    short_description TEXT,
    price DECIMAL(12,2) NOT NULL DEFAULT 0,
    regular_price DECIMAL(12,2),
    sale_price DECIMAL(12,2),
    currency VARCHAR(3) DEFAULT 'EUR',
    ean VARCHAR(50),
    sku VARCHAR(100),
    mpn VARCHAR(100),
    external_id VARCHAR(255),
    image_url TEXT,
    gallery_images JSONB DEFAULT '[]'::jsonb,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    category_path TEXT,
    brand VARCHAR(255),
    manufacturer VARCHAR(255),
    stock_status VARCHAR(50) DEFAULT 'instock',
    stock_quantity INTEGER,
    is_active BOOLEAN DEFAULT true,
    is_featured BOOLEAN DEFAULT false,
    attributes JSONB DEFAULT '{}'::jsonb,
    affiliate_url TEXT,
    button_text VARCHAR(100) DEFAULT 'Kúpiť',
    delivery_time VARCHAR(100),
    feed_id UUID REFERENCES feeds(id) ON DELETE SET NULL,
    feed_checksum VARCHAR(64),
    view_count INTEGER DEFAULT 0,
    click_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_products_slug ON products(slug);
CREATE INDEX IF NOT EXISTS idx_products_ean ON products(ean);
CREATE INDEX IF NOT EXISTS idx_products_sku ON products(sku);
CREATE INDEX IF NOT EXISTS idx_products_category ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_feed ON products(feed_id);
CREATE INDEX IF NOT EXISTS idx_products_active ON products(is_active);
CREATE INDEX IF NOT EXISTS idx_products_title_trgm ON products USING gin (title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_products_price ON products(price);

-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
-- IMPORT HISTORY
-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

CREATE TABLE IF NOT EXISTS import_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    feed_id UUID NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    finished_at TIMESTAMP WITH TIME ZONE,
    duration INTEGER DEFAULT 0,
    total_items INTEGER DEFAULT 0,
    processed INTEGER DEFAULT 0,
    created INTEGER DEFAULT 0,
    updated INTEGER DEFAULT 0,
    skipped INTEGER DEFAULT 0,
    errors INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'running',
    error_message TEXT,
    triggered_by VARCHAR(20) DEFAULT 'manual'
);

CREATE INDEX IF NOT EXISTS idx_import_history_feed ON import_history(feed_id);
CREATE INDEX IF NOT EXISTS idx_import_history_status ON import_history(status);

-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
-- SHOP CONFIG
-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

CREATE TABLE IF NOT EXISTS shop_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_name VARCHAR(255) DEFAULT 'My Shop',
    shop_url VARCHAR(255),
    logo TEXT,
    favicon TEXT,
    currency VARCHAR(3) DEFAULT 'EUR',
    locale VARCHAR(10) DEFAULT 'sk',
    template VARCHAR(50) DEFAULT 'aurora',
    primary_color VARCHAR(20) DEFAULT '#3B82F6',
    secondary_color VARCHAR(20) DEFAULT '#10B981',
    google_analytics VARCHAR(50),
    meta_title VARCHAR(255),
    meta_description TEXT,
    custom_css TEXT,
    custom_js TEXT,
    settings JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Insert default config
INSERT INTO shop_config (shop_name, template, primary_color, secondary_color)
VALUES ('EshopBuilder Store', 'aurora', '#3B82F6', '#10B981')
ON CONFLICT DO NOTHING;

-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
-- DEFAULT ADMIN USER
-- ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

-- Password: admin123 (bcrypt hash)
INSERT INTO users (email, password_hash, name, role)
VALUES ('admin@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'Admin', 'admin')
ON CONFLICT (email) DO NOTHING;
