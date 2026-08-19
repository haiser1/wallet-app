package unit

import (
	"testing"

	"test-teknis/internal/domain"
	appErrors "test-teknis/internal/errors"
	appValidator "test-teknis/internal/validator"
)

func TestCustomValidator_CreateUserRequest(t *testing.T) {
	v := appValidator.NewCustomValidator()

	tests := []struct {
		name    string
		req     domain.CreateUserRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid user request",
			req:     domain.CreateUserRequest{Username: "john_doe", Email: "john@example.com", Password: "password123"},
			wantErr: false,
		},
		{
			name:    "missing username",
			req:     domain.CreateUserRequest{Username: "", Email: "john@example.com", Password: "password123"},
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name:    "username too short",
			req:     domain.CreateUserRequest{Username: "ab", Email: "john@example.com", Password: "password123"},
			wantErr: true,
			errMsg:  "username must be at least 3 characters",
		},
		{
			name:    "missing email",
			req:     domain.CreateUserRequest{Username: "john_doe", Email: "", Password: "password123"},
			wantErr: true,
			errMsg:  "email is required",
		},
		{
			name:    "invalid email",
			req:     domain.CreateUserRequest{Username: "john_doe", Email: "not-an-email", Password: "password123"},
			wantErr: true,
			errMsg:  "invalid email format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(&tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestCustomValidator_TopUpRequest(t *testing.T) {
	v := appValidator.NewCustomValidator()

	tests := []struct {
		name    string
		req     domain.TopUpRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid topup request",
			req:     domain.TopUpRequest{Amount: 50000, IdempotencyKey: "key-123"},
			wantErr: false,
		},
		{
			name:    "zero amount",
			req:     domain.TopUpRequest{Amount: 0, IdempotencyKey: "key-123"},
			wantErr: true,
			errMsg:  appErrors.ErrInvalidAmount.Message,
		},
		{
			name:    "negative amount",
			req:     domain.TopUpRequest{Amount: -1000, IdempotencyKey: "key-123"},
			wantErr: true,
			errMsg:  appErrors.ErrInvalidAmount.Message,
		},
		{
			name:    "missing idempotency key",
			req:     domain.TopUpRequest{Amount: 50000, IdempotencyKey: ""},
			wantErr: true,
			errMsg:  appErrors.ErrIdempotencyKey.Message,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(&tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestCustomValidator_TransferRequest(t *testing.T) {
	v := appValidator.NewCustomValidator()
	validUUID1 := "11111111-1111-1111-1111-111111111111"
	validUUID2 := "22222222-2222-2222-2222-222222222222"

	tests := []struct {
		name    string
		req     domain.TransferRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid transfer request",
			req: domain.TransferRequest{
				FromUserID:     validUUID1,
				ToUserID:       validUUID2,
				Amount:         50000,
				IdempotencyKey: "key-123",
			},
			wantErr: false,
		},
		{
			name: "transfer to system wallet",
			req: domain.TransferRequest{
				FromUserID:     validUUID1,
				ToUserID:       domain.SystemWalletID,
				Amount:         50000,
				IdempotencyKey: "key-123",
			},
			wantErr: true,
			errMsg:  appErrors.ErrSystemWallet.Message,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(&tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}
