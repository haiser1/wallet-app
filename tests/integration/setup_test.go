package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"test-teknis/internal/database"
	"test-teknis/internal/repository"
	"test-teknis/internal/service"
)

var (
	testPool            *pgxpool.Pool
	testWalletService   *service.WalletService
	testUserService     *service.UserService
	testWalletRepo      *repository.WalletRepository
	testTxnRepo         *repository.TransactionRepository
	testIdempotencyRepo *repository.IdempotencyRepository
)

func TestMain(m *testing.M) {
	// Load .env from project root
	_ = godotenv.Load("../../.env")

	dbHost := getTestEnv("DB_HOST", "localhost")
	dbPort := getTestEnv("DB_PORT", "5432")
	dbUser := getTestEnv("DB_USER", "wallet_user")
	dbPass := getTestEnv("DB_PASSWORD", "wallet_pass")
	dbName := getTestEnv("DB_NAME", "wallet_db")
	dbSSL := getTestEnv("DB_SSLMODE", "disable")

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPass, dbHost, dbPort, dbName, dbSSL)

	var err error
	testPool, err = database.NewPostgresPool(dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to test database: %v", err)
	}
	defer testPool.Close()

	// Run migrations
	migrationSQL, err := os.ReadFile("../../internal/database/migrations/001_init.sql")
	if err != nil {
		log.Fatalf("Failed to read migration: %v", err)
	}
	if err := database.RunMigrations(testPool, string(migrationSQL)); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repos and services
	userRepo := repository.NewUserRepository(testPool)
	testWalletRepo = repository.NewWalletRepository(testPool)
	testTxnRepo = repository.NewTransactionRepository(testPool)
	testIdempotencyRepo = repository.NewIdempotencyRepository(testPool)

	testUserService = service.NewUserService(userRepo)
	testWalletService = service.NewWalletService(testPool, testWalletRepo, testTxnRepo, testIdempotencyRepo)

	os.Exit(m.Run())
}

// cleanupTestData removes all test data (except the system user/wallet).
func cleanupTestData(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	queries := []string{
		"DELETE FROM idempotency_keys",
		"DELETE FROM ledger_entries",
		"DELETE FROM transactions",
		"DELETE FROM wallets WHERE user_id != '00000000-0000-0000-0000-000000000000'",
		"DELETE FROM users WHERE id != '00000000-0000-0000-0000-000000000000'",
	}
	for _, q := range queries {
		if _, err := testPool.Exec(ctx, q); err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}
}

func getTestEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
