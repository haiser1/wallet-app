package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"test-teknis/internal/config"
	"test-teknis/internal/database"
	"test-teknis/internal/handler"
	"test-teknis/internal/repository"
	"test-teknis/internal/service"
)

func main() {
	// Load .env file (ignore error if not found, e.g. in production)
	_ = godotenv.Load()

	// Load configuration
	cfg := config.Load()

	// Connect to database
	pool, err := database.NewPostgresPool(cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations
	migrationSQL, err := os.ReadFile("internal/database/migrations/001_init.sql")
	if err != nil {
		log.Fatalf("Failed to read migration file: %v", err)
	}
	if err := database.RunMigrations(pool, string(migrationSQL)); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations applied successfully")

	// Initialize repositories
	userRepo := repository.NewUserRepository(pool)
	walletRepo := repository.NewWalletRepository(pool)
	txnRepo := repository.NewTransactionRepository(pool)
	idempotencyRepo := repository.NewIdempotencyRepository(pool)

	// Initialize services
	userService := service.NewUserService(userRepo)
	walletService := service.NewWalletService(pool, walletRepo, txnRepo, idempotencyRepo)

	// Initialize handlers
	userHandler := handler.NewUserHandler(userService)
	walletHandler := handler.NewWalletHandler(walletService)

	// Setup Echo
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(handler.RequestIDMiddleware())

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Register routes
	userGroup := e.Group("/api/v1/users")
	userHandler.RegisterRoutes(userGroup)
	walletHandler.RegisterRoutes(e)

	// Start server
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Server starting on %s", addr)
	if err := e.Start(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
