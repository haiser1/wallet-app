package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError is a structured application error with an HTTP status code.
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return e.Message
}

// Predefined application errors.
var (
	ErrInsufficientBalance = &AppError{Code: http.StatusBadRequest, Message: "insufficient balance"}
	ErrWalletNotFound      = &AppError{Code: http.StatusNotFound, Message: "wallet not found"}
	ErrUserNotFound        = &AppError{Code: http.StatusNotFound, Message: "user not found"}
	ErrSelfTransfer        = &AppError{Code: http.StatusBadRequest, Message: "cannot transfer to the same user"}
	ErrInvalidAmount       = &AppError{Code: http.StatusBadRequest, Message: "amount must be greater than zero"}
	ErrTransactionNotFound = &AppError{Code: http.StatusNotFound, Message: "transaction not found"}
	ErrAlreadyReversed     = &AppError{Code: http.StatusConflict, Message: "transaction has already been reversed"}
	ErrCannotReverse       = &AppError{Code: http.StatusBadRequest, Message: "only completed topup and transfer transactions can be reversed"}
	ErrDuplicateUser       = &AppError{Code: http.StatusConflict, Message: "username or email already exists"}
	ErrIdempotencyKey      = &AppError{Code: http.StatusBadRequest, Message: "idempotency key is required for this operation"}
	ErrSystemWallet        = &AppError{Code: http.StatusBadRequest, Message: "cannot perform operations on the system wallet"}
	ErrUnauthorized        = &AppError{Code: http.StatusUnauthorized, Message: "unauthorized: missing or invalid authentication token"}
	ErrForbidden           = &AppError{Code: http.StatusForbidden, Message: "forbidden: access denied to other user data"}
)

// NewValidationError creates a validation error with a custom message.
func NewValidationError(message string) *AppError {
	return &AppError{Code: http.StatusBadRequest, Message: message}
}

// NewInternalError creates an internal server error wrapping the original error.
func NewInternalError(err error) *AppError {
	return &AppError{Code: http.StatusInternalServerError, Message: "internal server error"}
}

// WrapError wraps an error with additional context.
func WrapError(msg string, err error) error {
	return fmt.Errorf("%s: %w", msg, err)
}

// IsAppError checks if an error is an AppError and returns it.
func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// ErrorResponse is the standard JSON error response body.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the error details.
type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
