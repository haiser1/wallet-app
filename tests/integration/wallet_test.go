package integration

import (
	"context"
	"fmt"
	"testing"

	"test-teknis/internal/domain"
	appValidator "test-teknis/internal/validator"
)

// TestTopUp_Success verifies that a top-up correctly increases the wallet balance
// and creates proper double-entry ledger entries.
func TestTopUp_Success(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	// Create a user
	user := createTestUser(t, domain.CreateUserRequest{
		Username: "topup_user",
		Email:    "topup@test.com",
	})

	// Top-up
	resp, err := testWalletService.TopUp(ctx, user.ID, domain.TopUpRequest{
		Amount:         100000, // 100,000 (smallest unit)
		IdempotencyKey: "topup-001",
	})
	if err != nil {
		t.Fatalf("top up: %v", err)
	}

	if resp.Balance != 100000 {
		t.Errorf("expected balance 100000, got %d", resp.Balance)
	}
	if resp.TransactionID == "" {
		t.Error("expected transaction ID")
	}

	// Verify wallet balance
	wallet, err := testWalletService.GetBalance(ctx, user.ID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if wallet.Balance != 100000 {
		t.Errorf("expected wallet balance 100000, got %d", wallet.Balance)
	}
}

// TestTopUp_Idempotency verifies that the same idempotency key returns the
// same response without creating duplicate entries.
func TestTopUp_Idempotency(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	user := createTestUser(t, domain.CreateUserRequest{
		Username: "idemp_user",
		Email:    "idemp@test.com",
	})

	key := "topup-idemp-001"

	// First request
	resp1, err := testWalletService.TopUp(ctx, user.ID, domain.TopUpRequest{
		Amount:         50000,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("first top up: %v", err)
	}

	// Second request with same key
	resp2, err := testWalletService.TopUp(ctx, user.ID, domain.TopUpRequest{
		Amount:         50000,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("second top up: %v", err)
	}

	// Should return the same transaction
	if resp1.TransactionID != resp2.TransactionID {
		t.Errorf("idempotency failed: got different transaction IDs %s and %s",
			resp1.TransactionID, resp2.TransactionID)
	}

	// Balance should be 50000, not 100000
	wallet, err := testWalletService.GetBalance(ctx, user.ID)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if wallet.Balance != 50000 {
		t.Errorf("expected balance 50000 (idempotent), got %d", wallet.Balance)
	}
}

// TestTransfer_Success verifies atomic transfer between two wallets.
func TestTransfer_Success(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	alice := createTestUser(t, domain.CreateUserRequest{
		Username: "alice",
		Email:    "alice@test.com",
	})
	bob := createTestUser(t, domain.CreateUserRequest{
		Username: "bob",
		Email:    "bob@test.com",
	})

	// Top-up Alice
	_, err := testWalletService.TopUp(ctx, alice.ID, domain.TopUpRequest{
		Amount:         200000,
		IdempotencyKey: "alice-topup-001",
	})
	if err != nil {
		t.Fatalf("top up alice: %v", err)
	}

	// Transfer from Alice to Bob
	resp, err := testWalletService.Transfer(ctx, domain.TransferRequest{
		FromUserID:     alice.ID,
		ToUserID:       bob.ID,
		Amount:         75000,
		IdempotencyKey: "transfer-001",
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}

	if resp.FromBalance != 125000 {
		t.Errorf("expected sender balance 125000, got %d", resp.FromBalance)
	}
	if resp.ToBalance != 75000 {
		t.Errorf("expected receiver balance 75000, got %d", resp.ToBalance)
	}

	// Verify balances
	aliceWallet, _ := testWalletService.GetBalance(ctx, alice.ID)
	bobWallet, _ := testWalletService.GetBalance(ctx, bob.ID)

	if aliceWallet.Balance != 125000 {
		t.Errorf("alice balance mismatch: expected 125000, got %d", aliceWallet.Balance)
	}
	if bobWallet.Balance != 75000 {
		t.Errorf("bob balance mismatch: expected 75000, got %d", bobWallet.Balance)
	}
}

// TestTransfer_InsufficientBalance verifies that transfer fails with insufficient balance.
func TestTransfer_InsufficientBalance(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	alice := createTestUser(t, domain.CreateUserRequest{
		Username: "alice",
		Email:    "alice@test.com",
	})
	bob := createTestUser(t, domain.CreateUserRequest{
		Username: "bob",
		Email:    "bob@test.com",
	})

	// Alice has 0 balance, try to transfer
	_, err := testWalletService.Transfer(ctx, domain.TransferRequest{
		FromUserID:     alice.ID,
		ToUserID:       bob.ID,
		Amount:         50000,
		IdempotencyKey: "transfer-insufficient",
	})
	if err == nil {
		t.Fatal("expected error for insufficient balance")
	}
	if err.Error() != "insufficient balance" {
		t.Errorf("expected 'insufficient balance' error, got: %v", err)
	}
}

// TestTransfer_SelfTransfer verifies that self-transfer is rejected.
func TestTransfer_SelfTransfer(t *testing.T) {
	cleanupTestData(t)
	v := appValidator.NewCustomValidator()

	user := createTestUser(t, domain.CreateUserRequest{
		Username: "self_user",
		Email:    "self@test.com",
	})

	req := domain.TransferRequest{
		FromUserID:     user.ID,
		ToUserID:       user.ID,
		Amount:         10000,
		IdempotencyKey: "self-transfer",
	}
	err := v.Validate(&req)
	if err == nil {
		t.Fatal("expected error for self-transfer")
	}
	if err.Error() != "cannot transfer to the same user" {
		t.Errorf("expected 'cannot transfer to the same user', got: %v", err)
	}
}

// TestTransfer_InvalidAmount verifies that zero and negative amounts are rejected.
func TestTransfer_InvalidAmount(t *testing.T) {
	cleanupTestData(t)
	v := appValidator.NewCustomValidator()

	alice := createTestUser(t, domain.CreateUserRequest{
		Username: "alice",
		Email:    "alice@test.com",
	})
	bob := createTestUser(t, domain.CreateUserRequest{
		Username: "bob",
		Email:    "bob@test.com",
	})

	tests := []struct {
		name   string
		amount int64
	}{
		{"zero amount", 0},
		{"negative amount", -10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := domain.TransferRequest{
				FromUserID:     alice.ID,
				ToUserID:       bob.ID,
				Amount:         tt.amount,
				IdempotencyKey: fmt.Sprintf("invalid-amount-%d", tt.amount),
			}
			err := v.Validate(&req)
			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
			if err.Error() != "amount must be greater than zero" {
				t.Errorf("expected 'amount must be greater than zero', got: %v", err)
			}
		})
	}
}

// TestTransfer_WalletNotFound verifies proper error when wallet doesn't exist.
func TestTransfer_WalletNotFound(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	user := createTestUser(t, domain.CreateUserRequest{
		Username: "user_exists",
		Email:    "exists@test.com",
	})

	_, err := testWalletService.Transfer(ctx, domain.TransferRequest{
		FromUserID:     user.ID,
		ToUserID:       "11111111-1111-1111-1111-111111111111", // non-existent
		Amount:         10000,
		IdempotencyKey: "wallet-not-found",
	})
	if err == nil {
		t.Fatal("expected error for non-existent wallet")
	}
	if err.Error() != "wallet not found" {
		t.Errorf("expected 'wallet not found', got: %v", err)
	}
}

// TestReversal_TopUp verifies that a top-up reversal creates counter-entries
// and restores the original balance.
func TestReversal_TopUp(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	user := createTestUser(t, domain.CreateUserRequest{
		Username: "reversal_user",
		Email:    "reversal@test.com",
	})

	// Top-up
	topUpResp, err := testWalletService.TopUp(ctx, user.ID, domain.TopUpRequest{
		Amount:         100000,
		IdempotencyKey: "topup-to-reverse",
	})
	if err != nil {
		t.Fatalf("top up: %v", err)
	}

	// Reverse the top-up
	revResp, err := testWalletService.Reverse(ctx, topUpResp.TransactionID, domain.ReverseRequest{
		IdempotencyKey: "reverse-topup",
	})
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}

	if revResp.OriginalTransactionID != topUpResp.TransactionID {
		t.Errorf("expected original txn ID %s, got %s",
			topUpResp.TransactionID, revResp.OriginalTransactionID)
	}

	// Balance should be back to 0
	wallet, _ := testWalletService.GetBalance(ctx, user.ID)
	if wallet.Balance != 0 {
		t.Errorf("expected balance 0 after reversal, got %d", wallet.Balance)
	}
}

// TestReversal_Transfer verifies that a transfer reversal restores both wallets.
func TestReversal_Transfer(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	alice := createTestUser(t, domain.CreateUserRequest{
		Username: "alice",
		Email:    "alice@test.com",
	})
	bob := createTestUser(t, domain.CreateUserRequest{
		Username: "bob",
		Email:    "bob@test.com",
	})

	// Top-up Alice
	_, _ = testWalletService.TopUp(ctx, alice.ID, domain.TopUpRequest{
		Amount:         200000,
		IdempotencyKey: "alice-topup",
	})

	// Transfer
	transferResp, _ := testWalletService.Transfer(ctx, domain.TransferRequest{
		FromUserID:     alice.ID,
		ToUserID:       bob.ID,
		Amount:         80000,
		IdempotencyKey: "transfer-to-reverse",
	})

	// Reverse the transfer
	_, err := testWalletService.Reverse(ctx, transferResp.TransactionID, domain.ReverseRequest{
		IdempotencyKey: "reverse-transfer",
	})
	if err != nil {
		t.Fatalf("reverse transfer: %v", err)
	}

	// Alice should be back to 200000
	aliceWallet, _ := testWalletService.GetBalance(ctx, alice.ID)
	if aliceWallet.Balance != 200000 {
		t.Errorf("expected alice balance 200000, got %d", aliceWallet.Balance)
	}

	// Bob should be back to 0
	bobWallet, _ := testWalletService.GetBalance(ctx, bob.ID)
	if bobWallet.Balance != 0 {
		t.Errorf("expected bob balance 0, got %d", bobWallet.Balance)
	}
}

// TestReversal_AlreadyReversed verifies that double-reversal is rejected.
func TestReversal_AlreadyReversed(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	user := createTestUser(t, domain.CreateUserRequest{
		Username: "double_rev",
		Email:    "double_rev@test.com",
	})

	topUpResp, _ := testWalletService.TopUp(ctx, user.ID, domain.TopUpRequest{
		Amount:         50000,
		IdempotencyKey: "topup-double-rev",
	})

	// First reversal
	_, _ = testWalletService.Reverse(ctx, topUpResp.TransactionID, domain.ReverseRequest{
		IdempotencyKey: "rev-1",
	})

	// Second reversal attempt
	_, err := testWalletService.Reverse(ctx, topUpResp.TransactionID, domain.ReverseRequest{
		IdempotencyKey: "rev-2",
	})
	if err == nil {
		t.Fatal("expected error for double reversal")
	}
	if err.Error() != "transaction has already been reversed" {
		t.Errorf("expected 'transaction has already been reversed', got: %v", err)
	}
}

// TestMutations_PaginationAndDateFilter verifies paginated mutation listing.
func TestMutations_PaginationAndDateFilter(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	user := createTestUser(t, domain.CreateUserRequest{
		Username: "mutation_user",
		Email:    "mutation@test.com",
	})

	// Create multiple top-ups
	for i := 1; i <= 5; i++ {
		_, err := testWalletService.TopUp(ctx, user.ID, domain.TopUpRequest{
			Amount:         int64(i * 10000),
			IdempotencyKey: fmt.Sprintf("topup-mutation-%d", i),
		})
		if err != nil {
			t.Fatalf("top up %d: %v", i, err)
		}
	}

	// Get mutations with pagination
	mutations, err := testWalletService.GetMutations(ctx, user.ID, domain.MutationQuery{
		Page:    1,
		PerPage: 3,
	})
	if err != nil {
		t.Fatalf("get mutations: %v", err)
	}

	if len(mutations.Data) != 3 {
		t.Errorf("expected 3 mutations on page 1, got %d", len(mutations.Data))
	}
	if mutations.TotalItems != 5 {
		t.Errorf("expected 5 total items, got %d", mutations.TotalItems)
	}
	if mutations.TotalPages != 2 {
		t.Errorf("expected 2 total pages, got %d", mutations.TotalPages)
	}

	// Get page 2
	mutations2, err := testWalletService.GetMutations(ctx, user.ID, domain.MutationQuery{
		Page:    2,
		PerPage: 3,
	})
	if err != nil {
		t.Fatalf("get mutations page 2: %v", err)
	}
	if len(mutations2.Data) != 2 {
		t.Errorf("expected 2 mutations on page 2, got %d", len(mutations2.Data))
	}
}

// TestReconciliation_BalanceMatchesMutations verifies that the recorded balance
// equals the computed balance from ledger entries.
func TestReconciliation_BalanceMatchesMutations(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	alice := createTestUser(t, domain.CreateUserRequest{
		Username: "alice",
		Email:    "alice@test.com",
	})
	bob := createTestUser(t, domain.CreateUserRequest{
		Username: "bob",
		Email:    "bob@test.com",
	})

	// Series of operations
	_, _ = testWalletService.TopUp(ctx, alice.ID, domain.TopUpRequest{
		Amount: 500000, IdempotencyKey: "recon-topup-1",
	})
	_, _ = testWalletService.TopUp(ctx, bob.ID, domain.TopUpRequest{
		Amount: 300000, IdempotencyKey: "recon-topup-2",
	})
	_, _ = testWalletService.Transfer(ctx, domain.TransferRequest{
		FromUserID: alice.ID, ToUserID: bob.ID,
		Amount: 150000, IdempotencyKey: "recon-transfer-1",
	})
	_, _ = testWalletService.Transfer(ctx, domain.TransferRequest{
		FromUserID: bob.ID, ToUserID: alice.ID,
		Amount: 50000, IdempotencyKey: "recon-transfer-2",
	})

	// Run reconciliation
	report, err := testWalletService.Reconcile(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if !report.IsHealthy {
		t.Errorf("expected healthy reconciliation, got discrepancies: %+v", report.Discrepancies)
	}
	if report.TotalWallets != 2 {
		t.Errorf("expected 2 wallets, got %d", report.TotalWallets)
	}
	if report.ConsistentWallets != 2 {
		t.Errorf("expected 2 consistent wallets, got %d", report.ConsistentWallets)
	}
}

// TestDoubleEntry_LedgerBalances verifies that every transaction creates proper
// debit and credit entries that balance out.
func TestDoubleEntry_LedgerBalances(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	user := createTestUser(t, domain.CreateUserRequest{
		Username: "ledger_user",
		Email:    "ledger@test.com",
	})

	// Top-up
	topUpResp, _ := testWalletService.TopUp(ctx, user.ID, domain.TopUpRequest{
		Amount:         100000,
		IdempotencyKey: "ledger-topup",
	})

	// Verify ledger entries for the top-up transaction
	entries, err := testTxnRepo.GetLedgerEntriesByTransactionID(ctx, topUpResp.TransactionID)
	if err != nil {
		t.Fatalf("get entries: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 ledger entries (debit+credit), got %d", len(entries))
	}

	// Verify one credit and one debit
	var hasCredit, hasDebit bool
	var totalCredit, totalDebit int64
	for _, e := range entries {
		switch e.EntryType {
		case domain.EntryTypeCredit:
			hasCredit = true
			totalCredit += e.Amount
		case domain.EntryTypeDebit:
			hasDebit = true
			totalDebit += e.Amount
		}
	}

	if !hasCredit || !hasDebit {
		t.Error("expected both credit and debit entries")
	}
	if totalCredit != totalDebit {
		t.Errorf("double-entry imbalance: credit=%d, debit=%d", totalCredit, totalDebit)
	}
}
