package handler

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"

	appErrors "test-teknis/internal/errors"
)

// respondError maps domain errors to HTTP responses with structured error bodies.
func respondError(c echo.Context, err error) error {
	if appErr, ok := appErrors.IsAppError(err); ok {
		return c.JSON(appErr.Code, appErrors.ErrorResponse{
			Error: appErrors.ErrorDetail{
				Code:    appErr.Code,
				Message: appErr.Message,
			},
		})
	}

	// Log unexpected errors
	log.Printf("unexpected error: %v", err)

	return c.JSON(http.StatusInternalServerError, appErrors.ErrorResponse{
		Error: appErrors.ErrorDetail{
			Code:    http.StatusInternalServerError,
			Message: "internal server error",
		},
	})
}

// RequestIDMiddleware adds a request ID to the context for tracing.
func RequestIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqID := c.Request().Header.Get("X-Request-ID")
			if reqID != "" {
				c.Response().Header().Set("X-Request-ID", reqID)
			}
			return next(c)
		}
	}
}
