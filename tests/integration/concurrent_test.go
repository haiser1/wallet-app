package integration

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/haiser1/wallet-app/internal/domain"
)

// TestConcurrent_TransfersNoNegativeBalance runs multiple concurrent transfers
// from the same wallet and verifies that the balance never goes negative and
// no mutations are duplicated.
func TestConcurrent_TransfersNoNegativeBalance(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	// Create sender and receiver
	sender := createTestUser(t, domain.CreateUserRequest{
		Username: "concurrent_sender",
		Email:    "sender@concurrent.com",
	})
	receiver := createTestUser(t, domain.CreateUserRequest{
		Username: "concurrent_receiver",
		Email:    "receiver@concurrent.com",
	})

	// Top-up sender with 100,000
	_, err := testWalletService.TopUp(ctx, sender.ID, domain.TopUpRequest{
		Amount:         100000,
		IdempotencyKey: "concurrent-topup",
	})
	if err != nil {
		t.Fatalf("top up: %v", err)
	}

	// Launch 20 concurrent transfers of 10,000 each.
	// Only 10 should succeed (100,000 / 10,000 = 10), the rest should fail
	// with insufficient balance.
	numGoroutines := 20
	transferAmount := int64(10000)

	var (
		wg           sync.WaitGroup
		successCount int64
		failCount    int64
	)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := testWalletService.Transfer(ctx, domain.TransferRequest{
				FromUserID:     sender.ID,
				ToUserID:       receiver.ID,
				Amount:         transferAmount,
				IdempotencyKey: fmt.Sprintf("concurrent-transfer-%d", idx),
			})
			if err != nil {
				atomic.AddInt64(&failCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent transfers: %d succeeded, %d failed", successCount, failCount)

	// Exactly 10 should succeed
	if successCount != 10 {
		t.Errorf("expected exactly 10 successful transfers, got %d", successCount)
	}
	if failCount != 10 {
		t.Errorf("expected 10 failed transfers, got %d", failCount)
	}

	// Sender balance should be exactly 0
	senderWallet, err := testWalletService.GetBalance(ctx, sender.ID)
	if err != nil {
		t.Fatalf("get sender balance: %v", err)
	}
	if senderWallet.Balance != 0 {
		t.Errorf("expected sender balance 0, got %d", senderWallet.Balance)
	}

	// Receiver balance should be exactly 100,000
	receiverWallet, err := testWalletService.GetBalance(ctx, receiver.ID)
	if err != nil {
		t.Fatalf("get receiver balance: %v", err)
	}
	if receiverWallet.Balance != 100000 {
		t.Errorf("expected receiver balance 100000, got %d", receiverWallet.Balance)
	}

	// Reconciliation should be healthy
	report, err := testWalletService.Reconcile(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !report.IsHealthy {
		t.Errorf("reconciliation failed after concurrent transfers: %+v", report.Discrepancies)
	}
}

// TestConcurrent_IdempotentTransfers verifies that concurrent identical requests
// (same idempotency key) result in only one execution.
// Some losing requests may fail during the race window (winner hasn't committed yet),
// but the critical invariant is: only ONE transfer is actually executed.
func TestConcurrent_IdempotentTransfers(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	sender := createTestUser(t, domain.CreateUserRequest{
		Username: "idemp_sender",
		Email:    "idemp_sender@test.com",
	})
	receiver := createTestUser(t, domain.CreateUserRequest{
		Username: "idemp_receiver",
		Email:    "idemp_receiver@test.com",
	})

	_, _ = testWalletService.TopUp(ctx, sender.ID, domain.TopUpRequest{
		Amount:         100000,
		IdempotencyKey: "idemp-topup",
	})

	// Send 10 concurrent requests with the SAME idempotency key
	numGoroutines := 10
	sameKey := "same-idemp-key"

	var (
		wg     sync.WaitGroup
		txnIDs sync.Map
	)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			resp, err := testWalletService.Transfer(ctx, domain.TransferRequest{
				FromUserID:     sender.ID,
				ToUserID:       receiver.ID,
				Amount:         25000,
				IdempotencyKey: sameKey,
			})
			if err == nil {
				txnIDs.Store(resp.TransactionID, true)
			}
			// Some may fail with "concurrent idempotency race: please retry"
			// if the winning transaction hasn't committed yet. That's OK.
		}()
	}

	wg.Wait()

	// Count unique transaction IDs - should be exactly 1
	uniqueCount := 0
	txnIDs.Range(func(_, _ interface{}) bool {
		uniqueCount++
		return true
	})
	if uniqueCount != 1 {
		t.Errorf("expected 1 unique transaction, got %d", uniqueCount)
	}

	// THE KEY INVARIANT: Balance should reflect only ONE transfer of 25,000
	senderWallet, _ := testWalletService.GetBalance(ctx, sender.ID)
	if senderWallet.Balance != 75000 {
		t.Errorf("expected sender balance 75000, got %d", senderWallet.Balance)
	}

	receiverWallet, _ := testWalletService.GetBalance(ctx, receiver.ID)
	if receiverWallet.Balance != 25000 {
		t.Errorf("expected receiver balance 25000, got %d", receiverWallet.Balance)
	}
}

// TestConcurrent_BidirectionalTransfers tests concurrent transfers in both directions
// between two wallets and verifies that the total money is conserved.
func TestConcurrent_BidirectionalTransfers(t *testing.T) {
	cleanupTestData(t)
	ctx := context.Background()

	alice := createTestUser(t, domain.CreateUserRequest{
		Username: "bidir_alice",
		Email:    "bidir_alice@test.com",
	})
	bob := createTestUser(t, domain.CreateUserRequest{
		Username: "bidir_bob",
		Email:    "bidir_bob@test.com",
	})

	// Top-up both with 500,000
	_, _ = testWalletService.TopUp(ctx, alice.ID, domain.TopUpRequest{
		Amount: 500000, IdempotencyKey: "bidir-topup-alice",
	})
	_, _ = testWalletService.TopUp(ctx, bob.ID, domain.TopUpRequest{
		Amount: 500000, IdempotencyKey: "bidir-topup-bob",
	})

	totalInitial := int64(1000000)

	// 20 concurrent transfers: 10 Alice→Bob, 10 Bob→Alice
	numEach := 10
	var wg sync.WaitGroup
	wg.Add(numEach * 2)

	for i := 0; i < numEach; i++ {
		// Alice → Bob
		go func(idx int) {
			defer wg.Done()
			testWalletService.Transfer(ctx, domain.TransferRequest{
				FromUserID:     alice.ID,
				ToUserID:       bob.ID,
				Amount:         10000,
				IdempotencyKey: fmt.Sprintf("bidir-ab-%d", idx),
			})
		}(i)

		// Bob → Alice
		go func(idx int) {
			defer wg.Done()
			testWalletService.Transfer(ctx, domain.TransferRequest{
				FromUserID:     bob.ID,
				ToUserID:       alice.ID,
				Amount:         10000,
				IdempotencyKey: fmt.Sprintf("bidir-ba-%d", idx),
			})
		}(i)
	}

	wg.Wait()

	// Total money must be conserved
	aliceWallet, _ := testWalletService.GetBalance(ctx, alice.ID)
	bobWallet, _ := testWalletService.GetBalance(ctx, bob.ID)
	totalFinal := aliceWallet.Balance + bobWallet.Balance

	if totalFinal != totalInitial {
		t.Errorf("money not conserved: initial=%d, final=%d (alice=%d, bob=%d)",
			totalInitial, totalFinal, aliceWallet.Balance, bobWallet.Balance)
	}

	// Both should be non-negative
	if aliceWallet.Balance < 0 {
		t.Errorf("alice has negative balance: %d", aliceWallet.Balance)
	}
	if bobWallet.Balance < 0 {
		t.Errorf("bob has negative balance: %d", bobWallet.Balance)
	}

	// Reconciliation
	report, _ := testWalletService.Reconcile(ctx)
	if !report.IsHealthy {
		t.Errorf("reconciliation unhealthy after bidirectional transfers: %+v", report.Discrepancies)
	}

	t.Logf("Bidirectional result: Alice=%d, Bob=%d, Total=%d", aliceWallet.Balance, bobWallet.Balance, totalFinal)
}
