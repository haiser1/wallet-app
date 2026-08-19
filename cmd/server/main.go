package main

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	echoSwagger "github.com/swaggo/echo-swagger"

	_ "test-teknis/docs"
	"test-teknis/internal/config"
	"test-teknis/internal/database"
	"test-teknis/internal/handler"
	appMiddleware "test-teknis/internal/middleware"
	"test-teknis/internal/repository"
	"test-teknis/internal/service"
	appValidator "test-teknis/internal/validator"
)

// @title Wallet & Transaction API
// @version 1.0
// @description RESTful API for Wallet and Financial Transactions with double-entry ledger, pessimistic locking, race-proof idempotency, and JWT authentication.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type 'Bearer ' followed by your JWT token.

func main() {
	// Setup zerolog logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// Load .env file (ignore error if not found, e.g. in production)
	_ = godotenv.Load()

	// Load configuration
	cfg := config.Load()

	// Connect to database
	pool, err := database.NewPostgresPool(cfg.DatabaseURL())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()

	// Run migrations
	migrationSQL, err := os.ReadFile("internal/database/migrations/001_init.sql")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read migration file")
	}
	if err := database.RunMigrations(pool, string(migrationSQL)); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}
	log.Info().Msg("Database migrations applied successfully")

	// Initialize repositories
	userRepo := repository.NewUserRepository(pool)
	walletRepo := repository.NewWalletRepository(pool)
	txnRepo := repository.NewTransactionRepository(pool)
	idempotencyRepo := repository.NewIdempotencyRepository(pool)

	// Initialize services
	userService := service.NewUserService(userRepo, cfg.JWTSecret)
	walletService := service.NewWalletService(pool, walletRepo, txnRepo, idempotencyRepo)

	// Initialize handlers
	userHandler := handler.NewUserHandler(userService)
	walletHandler := handler.NewWalletHandler(walletService)

	// Setup Echo
	e := echo.New()
	e.HideBanner = true

	// Register Custom Validator (go-playground/validator)
	e.Validator = appValidator.NewCustomValidator()

	// Global Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(appMiddleware.RequestIDMiddleware())

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Swagger UI route
	e.GET("/swagger/*", echoSwagger.WrapHandler)

	// Public auth & user routes
	api := e.Group("/api/v1")
	api.POST("/users", userHandler.CreateUser)
	api.POST("/auth/login", userHandler.Login)

	// Protected routes (require JWT authentication)
	protected := api.Group("", appMiddleware.JWTMiddleware(cfg.JWTSecret))
	protected.GET("/users/me", userHandler.GetUserProfile)

	// Wallet & Transfer routes
	walletHandler.RegisterRoutes(e, protected)

	// Start server
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Info().Msgf("Server starting on %s", addr)
	log.Info().Msgf("Swagger UI available at http://localhost%s/swagger/index.html", addr)
	if err := e.Start(addr); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
