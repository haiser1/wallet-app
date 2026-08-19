package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IdempotencyEntry represents a stored idempotency key with its cached response.
type IdempotencyEntry struct {
	Key           string
	TransactionID string
	ResponseCode  int
	ResponseBody  json.RawMessage
}

// IdempotencyRepository handles idempotency key storage and lookup.
type IdempotencyRepository struct {
	pool *pgxpool.Pool
}

// NewIdempotencyRepository creates a new IdempotencyRepository.
func NewIdempotencyRepository(pool *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{pool: pool}
}

// Get retrieves a cached idempotency entry by key. Returns nil if not found.
func (r *IdempotencyRepository) Get(ctx context.Context, key string) (*IdempotencyEntry, error) {
	var entry IdempotencyEntry
	err := r.pool.QueryRow(ctx,
		`SELECT key, transaction_id, response_code, response_body
		 FROM idempotency_keys
		 WHERE key = $1 AND expires_at > now()`,
		key,
	).Scan(&entry.Key, &entry.TransactionID, &entry.ResponseCode, &entry.ResponseBody)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get idempotency key: %w", err)
	}
	return &entry, nil
}

// Set stores an idempotency key with its response.
// Uses INSERT ... ON CONFLICT to handle race conditions where two identical
// requests arrive simultaneously — only the first one will be stored.
func (r *IdempotencyRepository) Set(ctx context.Context, tx pgx.Tx, entry *IdempotencyEntry) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO idempotency_keys (key, transaction_id, response_code, response_body)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (key) DO NOTHING`,
		entry.Key, entry.TransactionID, entry.ResponseCode, entry.ResponseBody,
	)
	if err != nil {
		return fmt.Errorf("set idempotency key: %w", err)
	}
	return nil
}

// GetWithinTx retrieves an idempotency entry within an existing transaction, using FOR UPDATE
// to serialize concurrent access to the same key.
func (r *IdempotencyRepository) GetWithinTx(ctx context.Context, tx pgx.Tx, key string) (*IdempotencyEntry, error) {
	var entry IdempotencyEntry
	err := tx.QueryRow(ctx,
		`SELECT key, transaction_id, response_code, response_body
		 FROM idempotency_keys
		 WHERE key = $1 AND expires_at > now()
		 FOR UPDATE`,
		key,
	).Scan(&entry.Key, &entry.TransactionID, &entry.ResponseCode, &entry.ResponseBody)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get idempotency key in tx: %w", err)
	}
	return &entry, nil
}
