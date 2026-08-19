package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"test-teknis/internal/domain"
	appErrors "test-teknis/internal/errors"
)

// TransactionRepository handles data access for transactions and ledger entries.
type TransactionRepository struct {
	pool *pgxpool.Pool
}

// NewTransactionRepository creates a new TransactionRepository.
func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

// CreateTransaction inserts a new transaction record within an existing transaction.
func (r *TransactionRepository) CreateTransaction(ctx context.Context, tx pgx.Tx, txn *domain.Transaction) error {
	var metadataJSON []byte
	var err error
	if txn.Metadata != nil {
		metadataJSON, err = json.Marshal(txn.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (type, reference_id, idempotency_key, status, metadata)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		txn.Type, txn.ReferenceID, txn.IdempotencyKey, txn.Status, metadataJSON,
	).Scan(&txn.ID, &txn.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

// CreateLedgerEntry inserts a new ledger entry within an existing transaction.
func (r *TransactionRepository) CreateLedgerEntry(ctx context.Context, tx pgx.Tx, entry *domain.LedgerEntry) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO ledger_entries (transaction_id, wallet_id, entry_type, amount, balance_after, description)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		entry.TransactionID, entry.WalletID, entry.EntryType, entry.Amount, entry.BalanceAfter, entry.Description,
	).Scan(&entry.ID, &entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert ledger entry: %w", err)
	}
	return nil
}

// GetTransactionByID retrieves a transaction by its ID.
func (r *TransactionRepository) GetTransactionByID(ctx context.Context, id string) (*domain.Transaction, error) {
	var txn domain.Transaction
	var metadataJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, type, reference_id, idempotency_key, status, metadata, created_at
		 FROM transactions WHERE id = $1`,
		id,
	).Scan(&txn.ID, &txn.Type, &txn.ReferenceID, &txn.IdempotencyKey, &txn.Status, &metadataJSON, &txn.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get transaction by id: %w", err)
	}
	if metadataJSON != nil {
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
			txn.Metadata = metadata
		}
	}
	return &txn, nil
}

// UpdateTransactionStatus updates the status of a transaction within a tx.
func (r *TransactionRepository) UpdateTransactionStatus(ctx context.Context, tx pgx.Tx, txnID string, status domain.TransactionStatus) error {
	_, err := tx.Exec(ctx,
		`UPDATE transactions SET status = $1 WHERE id = $2`,
		status, txnID,
	)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}
	return nil
}

// GetLedgerEntriesByTransactionID retrieves all ledger entries for a transaction.
func (r *TransactionRepository) GetLedgerEntriesByTransactionID(ctx context.Context, txnID string) ([]domain.LedgerEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, transaction_id, wallet_id, entry_type, amount, balance_after, description, created_at
		 FROM ledger_entries WHERE transaction_id = $1
		 ORDER BY created_at`,
		txnID,
	)
	if err != nil {
		return nil, fmt.Errorf("get ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.WalletID, &e.EntryType,
			&e.Amount, &e.BalanceAfter, &e.Description, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetMutations retrieves paginated ledger entries for a wallet with optional date filters.
func (r *TransactionRepository) GetMutations(ctx context.Context, query domain.MutationQuery) (*domain.PaginatedMutations, error) {
	// Build query dynamically
	baseWhere := "WHERE wallet_id = $1"
	args := []interface{}{query.WalletID}
	argIdx := 2

	if query.StartDate != nil {
		baseWhere += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *query.StartDate)
		argIdx++
	}
	if query.EndDate != nil {
		baseWhere += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *query.EndDate)
		argIdx++
	}

	// Count total
	var totalItems int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM ledger_entries %s", baseWhere)
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&totalItems); err != nil {
		return nil, fmt.Errorf("count mutations: %w", err)
	}

	// Fetch page
	offset := (query.Page - 1) * query.PerPage
	dataSQL := fmt.Sprintf(
		`SELECT id, transaction_id, wallet_id, entry_type, amount, balance_after, description, created_at
		 FROM ledger_entries %s
		 ORDER BY created_at DESC
		 LIMIT $%d OFFSET $%d`,
		baseWhere, argIdx, argIdx+1,
	)
	args = append(args, query.PerPage, offset)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("query mutations: %w", err)
	}
	defer rows.Close()

	var entries []domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.WalletID, &e.EntryType,
			&e.Amount, &e.BalanceAfter, &e.Description, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan mutation: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(query.PerPage)))
	if totalPages == 0 {
		totalPages = 1
	}

	return &domain.PaginatedMutations{
		Data:       entries,
		Page:       query.Page,
		PerPage:    query.PerPage,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}, nil
}

// GetComputedBalance calculates the balance from ledger entries for reconciliation.
// Credits add to balance, debits subtract.
func (r *TransactionRepository) GetComputedBalance(ctx context.Context, walletID string) (int64, error) {
	var creditSum, debitSum int64

	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount ELSE 0 END), 0)
		 FROM ledger_entries WHERE wallet_id = $1`,
		walletID,
	).Scan(&creditSum, &debitSum)
	if err != nil {
		return 0, fmt.Errorf("get computed balance: %w", err)
	}

	return creditSum - debitSum, nil
}
