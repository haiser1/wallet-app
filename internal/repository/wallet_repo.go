package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"test-teknis/internal/domain"
	appErrors "test-teknis/internal/errors"
)

// WalletRepository handles data access for wallets.
type WalletRepository struct {
	pool *pgxpool.Pool
}

// NewWalletRepository creates a new WalletRepository.
func NewWalletRepository(pool *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{pool: pool}
}

// GetByUserID retrieves a wallet by user ID.
func (r *WalletRepository) GetByUserID(ctx context.Context, userID string) (*domain.Wallet, error) {
	var wallet domain.Wallet
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, balance, currency, created_at, updated_at
		 FROM wallets WHERE user_id = $1`,
		userID,
	).Scan(&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency,
		&wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.ErrWalletNotFound
		}
		return nil, fmt.Errorf("get wallet by user id: %w", err)
	}
	return &wallet, nil
}

// GetByUserIDForUpdate retrieves a wallet by user ID with a pessimistic lock.
// Must be called within a transaction.
func (r *WalletRepository) GetByUserIDForUpdate(ctx context.Context, tx pgx.Tx, userID string) (*domain.Wallet, error) {
	var wallet domain.Wallet
	err := tx.QueryRow(ctx,
		`SELECT id, user_id, balance, currency, created_at, updated_at
		 FROM wallets WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency,
		&wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.ErrWalletNotFound
		}
		return nil, fmt.Errorf("get wallet for update: %w", err)
	}
	return &wallet, nil
}

// UpdateBalance sets the wallet balance and updated_at timestamp.
// Must be called within a transaction.
func (r *WalletRepository) UpdateBalance(ctx context.Context, tx pgx.Tx, walletID string, newBalance int64) error {
	_, err := tx.Exec(ctx,
		`UPDATE wallets SET balance = $1, updated_at = now() WHERE id = $2`,
		newBalance, walletID,
	)
	if err != nil {
		return fmt.Errorf("update wallet balance: %w", err)
	}
	return nil
}

// GetAllForReconciliation retrieves all non-system wallets for reconciliation.
func (r *WalletRepository) GetAllForReconciliation(ctx context.Context) ([]domain.Wallet, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, balance, currency, created_at, updated_at
		 FROM wallets WHERE user_id != $1
		 ORDER BY created_at`,
		domain.SystemWalletID,
	)
	if err != nil {
		return nil, fmt.Errorf("get all wallets: %w", err)
	}
	defer rows.Close()

	var wallets []domain.Wallet
	for rows.Next() {
		var w domain.Wallet
		if err := rows.Scan(&w.ID, &w.UserID, &w.Balance, &w.Currency,
			&w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan wallet: %w", err)
		}
		wallets = append(wallets, w)
	}
	return wallets, rows.Err()
}
