package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"test-teknis/internal/domain"
	appErrors "test-teknis/internal/errors"
	"test-teknis/internal/repository"
)

// WalletService handles wallet-related business logic including
// top-up, transfer, reversal, mutations, and reconciliation.
type WalletService struct {
	pool            *pgxpool.Pool
	walletRepo      *repository.WalletRepository
	txnRepo         *repository.TransactionRepository
	idempotencyRepo *repository.IdempotencyRepository
}

// NewWalletService creates a new WalletService.
func NewWalletService(
	pool *pgxpool.Pool,
	walletRepo *repository.WalletRepository,
	txnRepo *repository.TransactionRepository,
	idempotencyRepo *repository.IdempotencyRepository,
) *WalletService {
	return &WalletService{
		pool:            pool,
		walletRepo:      walletRepo,
		txnRepo:         txnRepo,
		idempotencyRepo: idempotencyRepo,
	}
}

// isUniqueViolation checks if an error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// GetBalance retrieves the current wallet balance for a user.
func (s *WalletService) GetBalance(ctx context.Context, userID string) (*domain.WalletBalanceResponse, error) {
	if userID == domain.SystemWalletID {
		return nil, appErrors.ErrSystemWallet
	}

	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &domain.WalletBalanceResponse{
		WalletID: wallet.ID,
		UserID:   wallet.UserID,
		Balance:  wallet.Balance,
		Currency: wallet.Currency,
	}, nil
}

// TopUp adds funds to a user's wallet.
// Creates a double-entry: credit to user wallet, debit from system wallet.
// The idempotency key prevents duplicate top-ups.
func (s *WalletService) TopUp(ctx context.Context, userID string, req domain.TopUpRequest) (*domain.TopUpResponse, error) {
	// --- Early validation ---
	if userID == domain.SystemWalletID {
		return nil, appErrors.ErrSystemWallet
	}
	if req.Amount <= 0 {
		return nil, appErrors.ErrInvalidAmount
	}
	if req.IdempotencyKey == "" {
		return nil, appErrors.ErrIdempotencyKey
	}

	// --- Check idempotency (fast path, outside transaction) ---
	existing, err := s.idempotencyRepo.Get(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		var resp domain.TopUpResponse
		if err := json.Unmarshal(existing.ResponseBody, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal cached response: %w", err)
		}
		return &resp, nil
	}

	// --- Begin transaction ---
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock wallet (pessimistic)
	wallet, err := s.walletRepo.GetByUserIDForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	// Calculate new balance
	newBalance := wallet.Balance + req.Amount

	// Create transaction record
	idempKey := req.IdempotencyKey
	txn := &domain.Transaction{
		Type:           domain.TransactionTypeTopUp,
		IdempotencyKey: &idempKey,
		Status:         domain.TransactionStatusCompleted,
		Metadata: map[string]interface{}{
			"user_id": userID,
			"amount":  req.Amount,
		},
	}
	if err := s.txnRepo.CreateTransaction(ctx, tx, txn); err != nil {
		// If unique constraint violation on idempotency_key, another concurrent
		// request won the race. Roll back and return the cached response.
		if isUniqueViolation(err) {
			tx.Rollback(ctx)
			return s.getIdempotentTopUpResponse(ctx, req.IdempotencyKey)
		}
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Create double-entry ledger entries:
	// 1. Credit to user wallet (balance increases)
	creditEntry := &domain.LedgerEntry{
		TransactionID: txn.ID,
		WalletID:      wallet.ID,
		EntryType:     domain.EntryTypeCredit,
		Amount:        req.Amount,
		BalanceAfter:  newBalance,
		Description:   "Top-up received",
	}
	if err := s.txnRepo.CreateLedgerEntry(ctx, tx, creditEntry); err != nil {
		return nil, fmt.Errorf("create credit entry: %w", err)
	}

	// 2. Debit from system wallet (source of funds)
	debitEntry := &domain.LedgerEntry{
		TransactionID: txn.ID,
		WalletID:      domain.SystemWalletID,
		EntryType:     domain.EntryTypeDebit,
		Amount:        req.Amount,
		BalanceAfter:  0, // System wallet is virtual; balance_after is nominal
		Description:   fmt.Sprintf("Top-up disbursement to user %s", userID),
	}
	if err := s.txnRepo.CreateLedgerEntry(ctx, tx, debitEntry); err != nil {
		return nil, fmt.Errorf("create debit entry: %w", err)
	}

	// Update wallet balance
	if err := s.walletRepo.UpdateBalance(ctx, tx, wallet.ID, newBalance); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	// Prepare response
	resp := &domain.TopUpResponse{
		TransactionID: txn.ID,
		Balance:       newBalance,
	}

	// Store idempotency key
	respJSON, _ := json.Marshal(resp)
	idempEntry := &repository.IdempotencyEntry{
		Key:           req.IdempotencyKey,
		TransactionID: txn.ID,
		ResponseCode:  200,
		ResponseBody:  respJSON,
	}
	if err := s.idempotencyRepo.Set(ctx, tx, idempEntry); err != nil {
		return nil, fmt.Errorf("store idempotency: %w", err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		// Commit failure due to serialization or unique violation also means
		// another request won; retry the idempotency lookup.
		if isUniqueViolation(err) {
			return s.getIdempotentTopUpResponse(ctx, req.IdempotencyKey)
		}
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return resp, nil
}

// getIdempotentTopUpResponse retries the idempotency lookup after a race condition.
func (s *WalletService) getIdempotentTopUpResponse(ctx context.Context, key string) (*domain.TopUpResponse, error) {
	existing, err := s.idempotencyRepo.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("retry idempotency check: %w", err)
	}
	if existing != nil {
		var resp domain.TopUpResponse
		if err := json.Unmarshal(existing.ResponseBody, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal cached response: %w", err)
		}
		return &resp, nil
	}
	// The winning transaction may not have committed yet; return the balance
	return nil, fmt.Errorf("concurrent idempotency race: please retry")
}

// Transfer moves funds from one user's wallet to another.
// Creates a double-entry for each wallet (2 ledger entries):
//   - Debit from sender
//   - Credit to receiver
//
// Wallets are locked in ascending user ID order to prevent deadlocks.
func (s *WalletService) Transfer(ctx context.Context, req domain.TransferRequest) (*domain.TransferResponse, error) {
	// --- Early validation ---
	if req.Amount <= 0 {
		return nil, appErrors.ErrInvalidAmount
	}
	if req.FromUserID == req.ToUserID {
		return nil, appErrors.ErrSelfTransfer
	}
	if req.FromUserID == domain.SystemWalletID || req.ToUserID == domain.SystemWalletID {
		return nil, appErrors.ErrSystemWallet
	}
	if req.IdempotencyKey == "" {
		return nil, appErrors.ErrIdempotencyKey
	}

	// --- Check idempotency (fast path) ---
	existing, err := s.idempotencyRepo.Get(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		var resp domain.TransferResponse
		if err := json.Unmarshal(existing.ResponseBody, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal cached response: %w", err)
		}
		return &resp, nil
	}

	// --- Determine lock order (ascending user ID to prevent deadlock) ---
	userIDs := []string{req.FromUserID, req.ToUserID}
	sort.Strings(userIDs)

	// --- Begin transaction ---
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock wallets in consistent order
	wallets := make(map[string]*domain.Wallet)
	for _, uid := range userIDs {
		w, err := s.walletRepo.GetByUserIDForUpdate(ctx, tx, uid)
		if err != nil {
			return nil, err
		}
		wallets[uid] = w
	}

	fromWallet := wallets[req.FromUserID]
	toWallet := wallets[req.ToUserID]

	// Check sufficient balance
	if fromWallet.Balance < req.Amount {
		return nil, appErrors.ErrInsufficientBalance
	}

	// Calculate new balances
	newFromBalance := fromWallet.Balance - req.Amount
	newToBalance := toWallet.Balance + req.Amount

	// Create transaction record
	idempKey := req.IdempotencyKey
	txn := &domain.Transaction{
		Type:           domain.TransactionTypeTransfer,
		IdempotencyKey: &idempKey,
		Status:         domain.TransactionStatusCompleted,
		Metadata: map[string]interface{}{
			"from_user_id": req.FromUserID,
			"to_user_id":   req.ToUserID,
			"amount":       req.Amount,
		},
	}
	if err := s.txnRepo.CreateTransaction(ctx, tx, txn); err != nil {
		// Unique constraint violation = another request with same key won the race
		if isUniqueViolation(err) {
			tx.Rollback(ctx)
			return s.getIdempotentTransferResponse(ctx, req.IdempotencyKey)
		}
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Create double-entry ledger entries:
	// 1. Debit from sender (balance decreases)
	debitEntry := &domain.LedgerEntry{
		TransactionID: txn.ID,
		WalletID:      fromWallet.ID,
		EntryType:     domain.EntryTypeDebit,
		Amount:        req.Amount,
		BalanceAfter:  newFromBalance,
		Description:   fmt.Sprintf("Transfer to user %s", req.ToUserID),
	}
	if err := s.txnRepo.CreateLedgerEntry(ctx, tx, debitEntry); err != nil {
		return nil, fmt.Errorf("create sender debit: %w", err)
	}

	// 2. Credit to receiver (balance increases)
	creditEntry := &domain.LedgerEntry{
		TransactionID: txn.ID,
		WalletID:      toWallet.ID,
		EntryType:     domain.EntryTypeCredit,
		Amount:        req.Amount,
		BalanceAfter:  newToBalance,
		Description:   fmt.Sprintf("Transfer from user %s", req.FromUserID),
	}
	if err := s.txnRepo.CreateLedgerEntry(ctx, tx, creditEntry); err != nil {
		return nil, fmt.Errorf("create receiver credit: %w", err)
	}

	// Update wallet balances
	if err := s.walletRepo.UpdateBalance(ctx, tx, fromWallet.ID, newFromBalance); err != nil {
		return nil, fmt.Errorf("update sender balance: %w", err)
	}
	if err := s.walletRepo.UpdateBalance(ctx, tx, toWallet.ID, newToBalance); err != nil {
		return nil, fmt.Errorf("update receiver balance: %w", err)
	}

	// Prepare response
	resp := &domain.TransferResponse{
		TransactionID: txn.ID,
		FromBalance:   newFromBalance,
		ToBalance:     newToBalance,
	}

	// Store idempotency key
	respJSON, _ := json.Marshal(resp)
	idempEntry := &repository.IdempotencyEntry{
		Key:           req.IdempotencyKey,
		TransactionID: txn.ID,
		ResponseCode:  200,
		ResponseBody:  respJSON,
	}
	if err := s.idempotencyRepo.Set(ctx, tx, idempEntry); err != nil {
		return nil, fmt.Errorf("store idempotency: %w", err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		if isUniqueViolation(err) {
			return s.getIdempotentTransferResponse(ctx, req.IdempotencyKey)
		}
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return resp, nil
}

// getIdempotentTransferResponse retries the idempotency lookup after a race condition.
func (s *WalletService) getIdempotentTransferResponse(ctx context.Context, key string) (*domain.TransferResponse, error) {
	existing, err := s.idempotencyRepo.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("retry idempotency check: %w", err)
	}
	if existing != nil {
		var resp domain.TransferResponse
		if err := json.Unmarshal(existing.ResponseBody, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal cached response: %w", err)
		}
		return &resp, nil
	}
	return nil, fmt.Errorf("concurrent idempotency race: please retry")
}

// Reverse creates reversal entries for a completed transaction.
// The original transaction is marked as 'reversed' and counter-entries are created.
// The original ledger entries are never modified or deleted (append-only).
func (s *WalletService) Reverse(ctx context.Context, txnID string, req domain.ReverseRequest) (*domain.ReverseResponse, error) {
	if req.IdempotencyKey == "" {
		return nil, appErrors.ErrIdempotencyKey
	}

	// Check idempotency (fast path)
	existing, err := s.idempotencyRepo.Get(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		var resp domain.ReverseResponse
		if err := json.Unmarshal(existing.ResponseBody, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal cached response: %w", err)
		}
		return &resp, nil
	}

	// Get original transaction
	origTxn, err := s.txnRepo.GetTransactionByID(ctx, txnID)
	if err != nil {
		return nil, err
	}

	// Validate reversibility
	if origTxn.Status == domain.TransactionStatusReversed {
		return nil, appErrors.ErrAlreadyReversed
	}
	if origTxn.Type != domain.TransactionTypeTopUp && origTxn.Type != domain.TransactionTypeTransfer {
		return nil, appErrors.ErrCannotReverse
	}

	// Get original ledger entries
	origEntries, err := s.txnRepo.GetLedgerEntriesByTransactionID(ctx, txnID)
	if err != nil {
		return nil, fmt.Errorf("get original entries: %w", err)
	}

	// Collect wallet IDs to lock (sorted for deadlock prevention)
	walletUserIDs := make(map[string]bool)
	for _, e := range origEntries {
		if e.WalletID != domain.SystemWalletID {
			walletUserIDs[e.WalletID] = true
		}
	}

	walletIDs := make([]string, 0, len(walletUserIDs))
	for wid := range walletUserIDs {
		walletIDs = append(walletIDs, wid)
	}
	sort.Strings(walletIDs)

	// Begin transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock wallets in sorted order by wallet ID
	lockedWallets := make(map[string]*domain.Wallet)
	for _, wid := range walletIDs {
		var wallet domain.Wallet
		err := tx.QueryRow(ctx,
			`SELECT id, user_id, balance, currency, created_at, updated_at
			 FROM wallets WHERE id = $1 FOR UPDATE`,
			wid,
		).Scan(&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency,
			&wallet.CreatedAt, &wallet.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("lock wallet %s: %w", wid, err)
		}
		lockedWallets[wid] = &wallet
	}

	// Create reversal transaction
	idempKey := req.IdempotencyKey
	reversalTxn := &domain.Transaction{
		Type:           domain.TransactionTypeReversal,
		ReferenceID:    &txnID,
		IdempotencyKey: &idempKey,
		Status:         domain.TransactionStatusCompleted,
		Metadata: map[string]interface{}{
			"original_transaction_id": txnID,
		},
	}
	if err := s.txnRepo.CreateTransaction(ctx, tx, reversalTxn); err != nil {
		if isUniqueViolation(err) {
			tx.Rollback(ctx)
			return s.getIdempotentReverseResponse(ctx, req.IdempotencyKey)
		}
		return nil, fmt.Errorf("create reversal transaction: %w", err)
	}

	// Create counter-entries (reverse each original entry)
	for _, origEntry := range origEntries {
		// Skip system wallet entries for balance calculation
		reverseType := domain.EntryTypeCredit
		if origEntry.EntryType == domain.EntryTypeCredit {
			reverseType = domain.EntryTypeDebit
		}

		var balanceAfter int64
		if origEntry.WalletID == domain.SystemWalletID {
			balanceAfter = 0
		} else {
			wallet := lockedWallets[origEntry.WalletID]
			if reverseType == domain.EntryTypeCredit {
				balanceAfter = wallet.Balance + origEntry.Amount
			} else {
				balanceAfter = wallet.Balance - origEntry.Amount
				if balanceAfter < 0 {
					return nil, appErrors.ErrInsufficientBalance
				}
			}
			// Update the tracked balance for subsequent entries
			wallet.Balance = balanceAfter
		}

		reversalEntry := &domain.LedgerEntry{
			TransactionID: reversalTxn.ID,
			WalletID:      origEntry.WalletID,
			EntryType:     reverseType,
			Amount:        origEntry.Amount,
			BalanceAfter:  balanceAfter,
			Description:   fmt.Sprintf("Reversal of transaction %s", txnID),
		}
		if err := s.txnRepo.CreateLedgerEntry(ctx, tx, reversalEntry); err != nil {
			return nil, fmt.Errorf("create reversal entry: %w", err)
		}
	}

	// Update wallet balances
	for _, wallet := range lockedWallets {
		if err := s.walletRepo.UpdateBalance(ctx, tx, wallet.ID, wallet.Balance); err != nil {
			return nil, fmt.Errorf("update wallet balance: %w", err)
		}
	}

	// Mark original transaction as reversed
	if err := s.txnRepo.UpdateTransactionStatus(ctx, tx, txnID, domain.TransactionStatusReversed); err != nil {
		return nil, fmt.Errorf("update original tx status: %w", err)
	}

	// Prepare response
	resp := &domain.ReverseResponse{
		ReversalTransactionID: reversalTxn.ID,
		OriginalTransactionID: txnID,
	}

	// Store idempotency key
	respJSON, _ := json.Marshal(resp)
	idempEntry := &repository.IdempotencyEntry{
		Key:           req.IdempotencyKey,
		TransactionID: reversalTxn.ID,
		ResponseCode:  200,
		ResponseBody:  respJSON,
	}
	if err := s.idempotencyRepo.Set(ctx, tx, idempEntry); err != nil {
		return nil, fmt.Errorf("store idempotency: %w", err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		if isUniqueViolation(err) {
			return s.getIdempotentReverseResponse(ctx, req.IdempotencyKey)
		}
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return resp, nil
}

// getIdempotentReverseResponse retries the idempotency lookup after a race condition.
func (s *WalletService) getIdempotentReverseResponse(ctx context.Context, key string) (*domain.ReverseResponse, error) {
	existing, err := s.idempotencyRepo.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("retry idempotency check: %w", err)
	}
	if existing != nil {
		var resp domain.ReverseResponse
		if err := json.Unmarshal(existing.ResponseBody, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal cached response: %w", err)
		}
		return &resp, nil
	}
	return nil, fmt.Errorf("concurrent idempotency race: please retry")
}

// GetMutations retrieves paginated ledger entries for a user's wallet.
func (s *WalletService) GetMutations(ctx context.Context, userID string, query domain.MutationQuery) (*domain.PaginatedMutations, error) {
	if userID == domain.SystemWalletID {
		return nil, appErrors.ErrSystemWallet
	}

	// Get wallet to obtain wallet ID
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	query.WalletID = wallet.ID

	// Set defaults
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PerPage <= 0 {
		query.PerPage = 20
	}
	if query.PerPage > 100 {
		query.PerPage = 100
	}

	return s.txnRepo.GetMutations(ctx, query)
}

// Reconcile checks the consistency of all wallet balances against their ledger entries.
func (s *WalletService) Reconcile(ctx context.Context) (*domain.ReconciliationReport, error) {
	wallets, err := s.walletRepo.GetAllForReconciliation(ctx)
	if err != nil {
		return nil, fmt.Errorf("get wallets for reconciliation: %w", err)
	}

	report := &domain.ReconciliationReport{
		TotalWallets: len(wallets),
		IsHealthy:    true,
	}

	for _, wallet := range wallets {
		computed, err := s.txnRepo.GetComputedBalance(ctx, wallet.ID)
		if err != nil {
			log.Printf("reconciliation error for wallet %s: %v", wallet.ID, err)
			continue
		}

		if wallet.Balance != computed {
			report.IsHealthy = false
			report.Discrepancies = append(report.Discrepancies, domain.ReconciliationResult{
				WalletID:        wallet.ID,
				UserID:          wallet.UserID,
				RecordedBalance: wallet.Balance,
				ComputedBalance: computed,
				IsConsistent:    false,
				Difference:      wallet.Balance - computed,
			})
		} else {
			report.ConsistentWallets++
		}
	}

	return report, nil
}
