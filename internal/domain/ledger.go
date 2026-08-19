package domain

import "time"

// EntryType represents whether a ledger entry is a debit or credit.
type EntryType string

const (
	EntryTypeDebit  EntryType = "debit"
	EntryTypeCredit EntryType = "credit"
)

// LedgerEntry represents a single debit or credit entry in the ledger.
// The ledger is append-only: entries are never modified or deleted.
type LedgerEntry struct {
	ID            string    `json:"id" db:"id"`
	TransactionID string    `json:"transaction_id" db:"transaction_id"`
	WalletID      string    `json:"wallet_id" db:"wallet_id"`
	EntryType     EntryType `json:"entry_type" db:"entry_type"`
	Amount        int64     `json:"amount" db:"amount"`
	BalanceAfter  int64     `json:"balance_after" db:"balance_after"`
	Description   string    `json:"description,omitempty" db:"description"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// MutationQuery holds filter and pagination parameters for listing mutations.
type MutationQuery struct {
	WalletID  string
	Page      int
	PerPage   int
	StartDate *time.Time
	EndDate   *time.Time
}

// PaginatedMutations is the paginated response for ledger entries.
type PaginatedMutations struct {
	Data       []LedgerEntry `json:"data"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalItems int64         `json:"total_items"`
	TotalPages int           `json:"total_pages"`
}

// ReconciliationResult holds the result of a balance reconciliation check.
type ReconciliationResult struct {
	WalletID        string `json:"wallet_id"`
	UserID          string `json:"user_id"`
	RecordedBalance int64  `json:"recorded_balance"`
	ComputedBalance int64  `json:"computed_balance"`
	IsConsistent    bool   `json:"is_consistent"`
	Difference      int64  `json:"difference,omitempty"`
}

// ReconciliationReport is the full report across all wallets.
type ReconciliationReport struct {
	TotalWallets      int                    `json:"total_wallets"`
	ConsistentWallets int                    `json:"consistent_wallets"`
	Discrepancies     []ReconciliationResult `json:"discrepancies,omitempty"`
	IsHealthy         bool                   `json:"is_healthy"`
}
