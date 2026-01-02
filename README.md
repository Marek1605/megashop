# 🚀 EshopBuilder Go API

Vysokovýkonné API pre e-commerce platformu napísané v Go.

## ⚡ Performance

| Metrika | Hodnota |
|---------|---------|
| Latencia | < 5ms |
| Throughput | 50,000+ req/s |
| RAM usage | ~50MB |
| Cold start | < 100ms |

## 🛠️ Tech Stack

- **Go 1.21** - jazyk
- **Fiber v2** - web framework (10× rýchlejší ako Express)
- **pgx** - natívny PostgreSQL driver
- **JWT** - autentifikácia
- **Docker** - kontajnerizácia

## 📦 Funkcie

- ✅ Registrácia/Prihlásenie (JWT)
- ✅ Multi-shop správa
- ✅ Produkty (CRUD, varianty, obrázky)
- ✅ Kategórie (hierarchické)
- ✅ Objednávky (stavy, história)
- ✅ Zákazníci
- ✅ Platby (GoPay, Stripe, ComGate)
- ✅ Doprava (viacero metód)
- ✅ Kupóny (% alebo fixné)
- ✅ Faktúry (automatické generovanie)
- ✅ Analytika (tržby, produkty)
- ✅ Import/Export produktov (CSV)

## 🚀 Spustenie

### Docker (odporúčané)

```bash
# Spusti všetko
docker-compose up -d

# API beží na http://localhost:8080
```

### Manuálne

```bash
# 1. Nainštaluj Go 1.21+
# 2. Nastav PostgreSQL
# 3. Skopíruj .env.example do .env a uprav

cp .env.example .env

# 4. Spusti
go run cmd/server/main.go
```

### V Coolify

1. Vytvor nový projekt: **Docker**
2. Git repository: nahraj tento kód
3. Build: `Dockerfile`
4. Environment variables: nastav podľa `.env.example`
5. Deploy!

## 🔧 Environment premenné

```env
# Povinné
DATABASE_URL=postgres://user:pass@host:5432/db
JWT_SECRET=min-32-znakov-secret-key
PORT=8080

# Voliteľné - platobné brány
GOPAY_CLIENT_ID=
GOPAY_CLIENT_SECRET=
GOPAY_GOID=
STRIPE_SECRET_KEY=
COMGATE_MERCHANT=
COMGATE_SECRET=
```

## 📚 API Dokumentácia

### Autentifikácia

```bash
# Registrácia
POST /api/v1/auth/register
{
  "email": "user@example.com",
  "password": "password123",
  "name": "Meno"
}

# Prihlásenie
POST /api/v1/auth/login
{
  "email": "user@example.com",
  "password": "password123"
}
# → { "token": "...", "refresh_token": "...", "user": {...} }
```

### Shopy

```bash
# Zoznam shopov (auth required)
GET /api/v1/shops
Authorization: Bearer <token>

# Vytvor shop
POST /api/v1/shops
{
  "name": "Môj E-shop",
  "currency": "EUR"
}
```

### Produkty

```bash
# Zoznam produktov
GET /api/v1/shops/{shopId}/products?page=1&limit=20&search=

# Vytvor produkt
POST /api/v1/shops/{shopId}/products
{
  "name": "Produkt",
  "price": 29.99,
  "quantity": 100,
  "images": [{"url": "https://..."}]
}

# Export CSV
GET /api/v1/shops/{shopId}/products/export?format=csv
```

### Objednávky

```bash
# Zoznam objednávok
GET /api/v1/shops/{shopId}/orders?status=pending

# Detail objednávky
GET /api/v1/shops/{shopId}/orders/{orderId}

# Aktualizuj stav
PUT /api/v1/shops/{shopId}/orders/{orderId}
{
  "status": "shipped",
  "tracking_number": "SK123456789"
}
```

### Verejné API (storefront)

```bash
# Shop info
GET /api/v1/shop/{slug}

# Produkty
GET /api/v1/shop/{slug}/products

# Vytvor objednávku
POST /api/v1/shop/{slug}/orders
{
  "items": [{"product_id": "...", "quantity": 2}],
  "shipping": {"first_name": "Ján", ...},
  "billing": {"email": "jan@example.com", ...},
  "shipping_method": "GLS",
  "payment_method": "card"
}
```

## 📊 Healthcheck

```bash
GET /health
# → { "status": "ok", "version": "1.0.0" }
```

## 🔒 Webhooky

```
POST /api/v1/webhooks/gopay
POST /api/v1/webhooks/stripe
POST /api/v1/webhooks/comgate
```

## 📁 Štruktúra projektu

```
├── cmd/
│   └── server/
│       └── main.go          # Entry point
├── internal/
│   ├── database/
│   │   └── database.go      # DB connection & migrations
│   ├── handlers/
│   │   ├── auth.go          # Auth handlers
│   │   ├── shops.go         # Shop handlers
│   │   ├── products.go      # Product handlers
│   │   ├── orders.go        # Order handlers
│   │   ├── payments.go      # Payment handlers
│   │   └── other.go         # Other handlers
│   ├── middleware/
│   │   └── jwt.go           # JWT middleware
│   └── models/
│       └── models.go        # Data models
├── templates/
│   └── invoice.html         # Invoice template
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── README.md
```

## ⚡ Benchmark

```
# 10M produktov, CCX43 server (8 vCPU, 32GB RAM)
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/shop/test/products

Requests/sec: 52,000
Latency avg:  3.2ms
Latency 99%:  12ms
```

## 📄 Licencia

MIT
