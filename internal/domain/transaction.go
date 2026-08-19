package domain

import "time"

// TransactionType represents the type of a financial transaction.
type TransactionType string

const (
	TransactionTypeTopUp    TransactionType = "topup"
	TransactionTypeTransfer TransactionType = "transfer"
	TransactionTypeReversal TransactionType = "reversal"
)

// TransactionStatus represents the current status of a transaction.
type TransactionStatus string

const (
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusReversed  TransactionStatus = "reversed"
)

// Transaction represents a financial transaction that groups ledger entries.
type Transaction struct {
	ID             string            `json:"id" db:"id"`
	Type           TransactionType   `json:"type" db:"type"`
	ReferenceID    *string           `json:"reference_id,omitempty" db:"reference_id"`
	IdempotencyKey *string           `json:"idempotency_key,omitempty" db:"idempotency_key"`
	Status         TransactionStatus `json:"status" db:"status"`
	Metadata       interface{}       `json:"metadata,omitempty" db:"metadata"`
	CreatedAt      time.Time         `json:"created_at" db:"created_at"`
}
