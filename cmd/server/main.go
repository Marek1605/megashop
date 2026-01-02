package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"eshop-builder/internal/database"
	"eshop-builder/internal/handlers"
	"eshop-builder/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	godotenv.Load()

	// Connect to database
	if err := database.Connect(); err != nil {
		log.Fatal("Database connection failed:", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.Migrate(); err != nil {
		log.Fatal("Migration failed:", err)
	}

	// Create Fiber app with optimized config
	app := fiber.New(fiber.Config{
		Prefork:       false, // Enable in production for multi-core
		ServerHeader:  "EshopBuilder",
		StrictRouting: true,
		CaseSensitive: true,
		BodyLimit:     50 * 1024 * 1024, // 50MB
		ReadBufferSize: 8192,
		Concurrency:   256 * 1024, // Max concurrent connections
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${status} | ${latency} | ${method} ${path}\n",
	}))
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Rate limiter
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 60,
	}))

	// Static files
	app.Static("/static", "./static")

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "version": "1.0.0"})
	})

	// Dashboard - serve index.html on root
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./static/index.html")
	})

	// API routes
	api := app.Group("/api/v1")

	// Public routes
	api.Post("/auth/register", handlers.Register)
	api.Post("/auth/login", handlers.Login)
	api.Post("/auth/refresh", handlers.RefreshToken)

	// Public shop routes (for storefront)
	api.Get("/shop/:slug", handlers.GetPublicShop)
	api.Get("/shop/:slug/products", handlers.GetPublicProducts)
	api.Get("/shop/:slug/product/:productSlug", handlers.GetPublicProduct)
	api.Get("/shop/:slug/categories", handlers.GetPublicCategories)
	api.Post("/shop/:slug/orders", handlers.CreatePublicOrder)
	api.Get("/shop/:slug/shipping-methods", handlers.GetPublicShippingMethods)
	api.Get("/shop/:slug/payment-methods", handlers.GetPublicPaymentMethods)

	// Payment webhooks (no auth)
	webhooks := api.Group("/webhooks")
	webhooks.Post("/gopay", handlers.GoPayWebhook)
	webhooks.Post("/stripe", handlers.StripeWebhook)
	webhooks.Post("/comgate", handlers.ComGateWebhook)

	// Protected routes
	protected := api.Group("/", middleware.JWTAuth())

	// User
	protected.Get("/me", handlers.GetCurrentUser)
	protected.Put("/me", handlers.UpdateCurrentUser)

	// Shops
	protected.Get("/shops", handlers.GetShops)
	protected.Post("/shops", handlers.CreateShop)
	protected.Get("/shops/:id", handlers.GetShop)
	protected.Put("/shops/:id", handlers.UpdateShop)
	protected.Delete("/shops/:id", handlers.DeleteShop)
	protected.Get("/shops/:id/stats", handlers.GetShopStats)

	// Products
	protected.Get("/shops/:shopId/products", handlers.GetProducts)
	protected.Post("/shops/:shopId/products", handlers.CreateProduct)
	protected.Get("/shops/:shopId/products/:id", handlers.GetProduct)
	protected.Put("/shops/:shopId/products/:id", handlers.UpdateProduct)
	protected.Delete("/shops/:shopId/products/:id", handlers.DeleteProduct)
	protected.Post("/shops/:shopId/products/import", handlers.ImportProducts)
	protected.Get("/shops/:shopId/products/export", handlers.ExportProducts)

	// Categories
	protected.Get("/shops/:shopId/categories", handlers.GetCategories)
	protected.Post("/shops/:shopId/categories", handlers.CreateCategory)
	protected.Put("/shops/:shopId/categories/:id", handlers.UpdateCategory)
	protected.Delete("/shops/:shopId/categories/:id", handlers.DeleteCategory)

	// Orders
	protected.Get("/shops/:shopId/orders", handlers.GetOrders)
	protected.Get("/shops/:shopId/orders/:id", handlers.GetOrder)
	protected.Put("/shops/:shopId/orders/:id", handlers.UpdateOrder)
	protected.Delete("/shops/:shopId/orders/:id", handlers.CancelOrder)
	protected.Post("/shops/:shopId/orders/:id/invoice", handlers.GenerateInvoice)

	// Customers
	protected.Get("/shops/:shopId/customers", handlers.GetCustomers)
	protected.Get("/shops/:shopId/customers/:id", handlers.GetCustomer)
	protected.Put("/shops/:shopId/customers/:id", handlers.UpdateCustomer)

	// Shipping methods
	protected.Get("/shops/:shopId/shipping-methods", handlers.GetShippingMethods)
	protected.Post("/shops/:shopId/shipping-methods", handlers.CreateShippingMethod)
	protected.Put("/shops/:shopId/shipping-methods/:id", handlers.UpdateShippingMethod)
	protected.Delete("/shops/:shopId/shipping-methods/:id", handlers.DeleteShippingMethod)

	// Payment methods
	protected.Get("/shops/:shopId/payment-methods", handlers.GetPaymentMethods)
	protected.Post("/shops/:shopId/payment-methods", handlers.CreatePaymentMethod)
	protected.Put("/shops/:shopId/payment-methods/:id", handlers.UpdatePaymentMethod)
	protected.Delete("/shops/:shopId/payment-methods/:id", handlers.DeletePaymentMethod)

	// Coupons
	protected.Get("/shops/:shopId/coupons", handlers.GetCoupons)
	protected.Post("/shops/:shopId/coupons", handlers.CreateCoupon)
	protected.Put("/shops/:shopId/coupons/:id", handlers.UpdateCoupon)
	protected.Delete("/shops/:shopId/coupons/:id", handlers.DeleteCoupon)

	// Analytics
	protected.Get("/shops/:shopId/analytics", handlers.GetAnalytics)
	protected.Get("/shops/:shopId/analytics/revenue", handlers.GetRevenueAnalytics)
	protected.Get("/shops/:shopId/analytics/products", handlers.GetProductAnalytics)

	// Settings
	protected.Get("/shops/:shopId/settings", handlers.GetSettings)
	protected.Put("/shops/:shopId/settings", handlers.UpdateSettings)

	// Invoices
	protected.Get("/shops/:shopId/invoices", handlers.GetInvoices)
	protected.Get("/shops/:shopId/invoices/:id", handlers.GetInvoice)
	protected.Get("/shops/:shopId/invoices/:id/pdf", handlers.GetInvoicePDF)

	// Payments
	protected.Post("/payments/gopay", handlers.CreateGoPayPayment)
	protected.Post("/payments/stripe", handlers.CreateStripePayment)
	protected.Post("/payments/comgate", handlers.CreateComGatePayment)
	protected.Get("/payments/:id/status", handlers.GetPaymentStatus)

	// Graceful shutdown
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server in goroutine
	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Panic(err)
		}
	}()

	log.Printf("🚀 EshopBuilder API running on port %s", port)

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Gracefully shutting down...")
	app.Shutdown()
}
