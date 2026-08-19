package unit

import (
	"testing"

	appErrors "test-teknis/internal/errors"
)

// TestAppError_IsAppError verifies error type detection.
func TestAppError_IsAppError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"insufficient balance", appErrors.ErrInsufficientBalance, true},
		{"wallet not found", appErrors.ErrWalletNotFound, true},
		{"self transfer", appErrors.ErrSelfTransfer, true},
		{"invalid amount", appErrors.ErrInvalidAmount, true},
		{"idempotency key", appErrors.ErrIdempotencyKey, true},
		{"system wallet", appErrors.ErrSystemWallet, true},
		{"already reversed", appErrors.ErrAlreadyReversed, true},
		{"cannot reverse", appErrors.ErrCannotReverse, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr, ok := appErrors.IsAppError(tt.err)
			if ok != tt.expected {
				t.Errorf("IsAppError(%v) = %v, want %v", tt.err, ok, tt.expected)
			}
			if ok && appErr.Code == 0 {
				t.Error("AppError should have a non-zero HTTP code")
			}
			if ok && appErr.Message == "" {
				t.Error("AppError should have a non-empty message")
			}
		})
	}
}

// TestAppError_ErrorMessages verifies that error messages are descriptive.
func TestAppError_ErrorMessages(t *testing.T) {
	tests := []struct {
		err      *appErrors.AppError
		expected string
	}{
		{appErrors.ErrInsufficientBalance, "insufficient balance"},
		{appErrors.ErrWalletNotFound, "wallet not found"},
		{appErrors.ErrSelfTransfer, "cannot transfer to the same user"},
		{appErrors.ErrInvalidAmount, "amount must be greater than zero"},
		{appErrors.ErrIdempotencyKey, "idempotency key is required for this operation"},
		{appErrors.ErrSystemWallet, "cannot perform operations on the system wallet"},
		{appErrors.ErrAlreadyReversed, "transaction has already been reversed"},
		{appErrors.ErrCannotReverse, "only completed topup and transfer transactions can be reversed"},
		{appErrors.ErrDuplicateUser, "username or email already exists"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.expected)
			}
		})
	}
}

// TestAppError_HTTPCodes verifies correct HTTP status codes for each error.
func TestAppError_HTTPCodes(t *testing.T) {
	tests := []struct {
		name string
		err  *appErrors.AppError
		code int
	}{
		{"insufficient balance → 400", appErrors.ErrInsufficientBalance, 400},
		{"wallet not found → 404", appErrors.ErrWalletNotFound, 404},
		{"user not found → 404", appErrors.ErrUserNotFound, 404},
		{"self transfer → 400", appErrors.ErrSelfTransfer, 400},
		{"invalid amount → 400", appErrors.ErrInvalidAmount, 400},
		{"already reversed → 409", appErrors.ErrAlreadyReversed, 409},
		{"duplicate user → 409", appErrors.ErrDuplicateUser, 409},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("expected HTTP %d, got %d", tt.code, tt.err.Code)
			}
		})
	}
}

// TestValidationError verifies custom validation error creation.
func TestValidationError(t *testing.T) {
	err := appErrors.NewValidationError("custom message")
	if err.Code != 400 {
		t.Errorf("expected 400, got %d", err.Code)
	}
	if err.Message != "custom message" {
		t.Errorf("expected 'custom message', got %q", err.Message)
	}
}
