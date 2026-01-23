package config

import "os"

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	Environment string
}

func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/eshopbuilder?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "super-secret-jwt-key-change-in-production"),
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
