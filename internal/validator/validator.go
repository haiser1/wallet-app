package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"

	appErrors "github.com/haiser1/wallet-app/internal/errors"
)

// CustomValidator wraps go-playground/validator/v10 for Echo request validation.
type CustomValidator struct {
	validator *validator.Validate
}

// NewCustomValidator creates a new CustomValidator instance.
func NewCustomValidator() *CustomValidator {
	return &CustomValidator{validator: validator.New()}
}

// Validate validates a struct and formats validation errors into AppError.
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			return FormatValidationError(ve)
		}
		return appErrors.NewValidationError(err.Error())
	}
	return nil
}

// FormatValidationError translates go-playground validation errors into domain AppErrors.
func FormatValidationError(ve validator.ValidationErrors) error {
	for _, fe := range ve {
		field := fe.Field()
		tag := fe.Tag()
		param := fe.Param()

		if tag == "ne" && param == "00000000-0000-0000-0000-000000000000" {
			return appErrors.ErrSystemWallet
		}

		switch field {
		case "Amount":
			if tag == "gt" || tag == "required" {
				return appErrors.ErrInvalidAmount
			}
		case "ToUserID":
			if tag == "nefield" {
				return appErrors.ErrSelfTransfer
			}
		case "IdempotencyKey":
			if tag == "required" {
				return appErrors.ErrIdempotencyKey
			}
		case "Username":
			if tag == "required" {
				return appErrors.NewValidationError("username is required")
			}
			if tag == "min" {
				return appErrors.NewValidationError("username must be at least 3 characters")
			}
		case "Email":
			if tag == "required" {
				return appErrors.NewValidationError("email is required")
			}
			if tag == "email" {
				return appErrors.NewValidationError("invalid email format")
			}
		}

		switch tag {
		case "required":
			return appErrors.NewValidationError(fmt.Sprintf("%s is required", strings.ToLower(field)))
		case "uuid":
			return appErrors.NewValidationError(fmt.Sprintf("%s must be a valid UUID", strings.ToLower(field)))
		case "gt":
			return appErrors.NewValidationError(fmt.Sprintf("%s must be greater than zero", strings.ToLower(field)))
		default:
			return appErrors.NewValidationError(fmt.Sprintf("%s validation failed on '%s'", strings.ToLower(field), tag))
		}
	}
	return appErrors.NewValidationError("validation failed")
}
