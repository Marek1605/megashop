package main

import (
	"log"
	"net/http"
	"os"

	"eshopbuilder/internal/config"
	"eshopbuilder/internal/database"
	"eshopbuilder/internal/handlers"
	"eshopbuilder/internal/middleware"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	// Load config
	cfg := config.Load()

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(db); err != nil {
		log.Printf("Migration warning: %v", err)
	}

	// Create handler
	h := handlers.New(db, cfg)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// API Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth
		r.Post("/auth/login", h.Login)
		r.Post("/auth/register", h.Register)
		r.Post("/auth/refresh", h.RefreshToken)

		// Public routes
		r.Get("/products", h.ListProducts)
		r.Get("/products/{slug}", h.GetProduct)
		r.Get("/categories", h.ListCategories)
		r.Get("/categories/{slug}", h.GetCategory)
		r.Get("/search", h.SearchProducts)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(cfg.JWTSecret))

			// Admin
			r.Route("/admin", func(r chi.Router) {
				// Dashboard
				r.Get("/stats", h.GetDashboardStats)
				r.Get("/recent-activity", h.GetRecentActivity)

				// Products
				r.Get("/products", h.AdminListProducts)
				r.Post("/products", h.CreateProduct)
				r.Get("/products/{id}", h.AdminGetProduct)
				r.Put("/products/{id}", h.UpdateProduct)
				r.Delete("/products/{id}", h.DeleteProduct)
				r.Post("/products/bulk-action", h.BulkProductAction)

				// Categories
				r.Get("/categories", h.AdminListCategories)
				r.Post("/categories", h.CreateCategory)
				r.Put("/categories/{id}", h.UpdateCategory)
				r.Delete("/categories/{id}", h.DeleteCategory)

				// Feeds
				r.Get("/feeds", h.ListFeeds)
				r.Post("/feeds", h.CreateFeed)
				r.Get("/feeds/{id}", h.GetFeed)
				r.Put("/feeds/{id}", h.UpdateFeed)
				r.Delete("/feeds/{id}", h.DeleteFeed)
				r.Post("/feeds/{id}/import", h.StartImport)
				r.Post("/feeds/{id}/stop", h.StopImport)
				r.Get("/feeds/{id}/progress", h.GetImportProgress)
				r.Get("/feeds/{id}/history", h.GetImportHistory)
				r.Post("/feeds/preview", h.PreviewFeed)
				r.Post("/feeds/auto-mapping", h.AutoMapping)

				// Settings
				r.Get("/settings", h.GetSettings)
				r.Put("/settings", h.UpdateSettings)

				// Shop config
				r.Get("/shop-config", h.GetShopConfig)
				r.Put("/shop-config", h.UpdateShopConfig)
			})
		})
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Get port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 EshopBuilder v3 API starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
