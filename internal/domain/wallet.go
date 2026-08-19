package domain

import "time"

// SystemWalletID is the well-known UUID for the system wallet used as counter-party in top-ups.
const SystemWalletID = "00000000-0000-0000-0000-000000000000"

// Wallet represents a user's wallet with a balance in the smallest currency unit.
type Wallet struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Balance   int64     `json:"balance" db:"balance"`
	Currency  string    `json:"currency" db:"currency"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TopUpRequest is the input for topping up a wallet.
type TopUpRequest struct {
	Amount         int64  `json:"amount" validate:"required,gt=0"`
	IdempotencyKey string `json:"-"` // From header
}

// TopUpResponse is returned after a successful top-up.
type TopUpResponse struct {
	TransactionID string `json:"transaction_id"`
	Balance       int64  `json:"balance"`
}

// TransferRequest is the input for transferring between wallets.
type TransferRequest struct {
	FromUserID     string `json:"from_user_id" validate:"required,uuid"`
	ToUserID       string `json:"to_user_id" validate:"required,uuid"`
	Amount         int64  `json:"amount" validate:"required,gt=0"`
	IdempotencyKey string `json:"-"` // From header
}

// TransferResponse is returned after a successful transfer.
type TransferResponse struct {
	TransactionID string `json:"transaction_id"`
	FromBalance   int64  `json:"from_balance"`
	ToBalance     int64  `json:"to_balance"`
}

// WalletBalanceResponse returns the wallet info to the client.
type WalletBalanceResponse struct {
	WalletID  string `json:"wallet_id"`
	UserID    string `json:"user_id"`
	Balance   int64  `json:"balance"`
	Currency  string `json:"currency"`
}

// ReverseRequest is the input for reversing a transaction.
type ReverseRequest struct {
	IdempotencyKey string `json:"-"` // From header
}

// ReverseResponse is returned after a successful reversal.
type ReverseResponse struct {
	ReversalTransactionID string `json:"reversal_transaction_id"`
	OriginalTransactionID string `json:"original_transaction_id"`
}
