package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"test-teknis/internal/auth"
	appErrors "test-teknis/internal/errors"
)

// RespondError maps domain errors to HTTP responses with structured error bodies.
func RespondError(c echo.Context, err error) error {
	if appErr, ok := appErrors.IsAppError(err); ok {
		return c.JSON(appErr.Code, appErrors.ErrorResponse{
			Error: appErrors.ErrorDetail{
				Code:    appErr.Code,
				Message: appErr.Message,
			},
		})
	}

	// Log unexpected errors
	log.Error().Err(err).Msg("unexpected error")

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

// JWTMiddleware validates the Bearer JWT token from the Authorization header and stores user_id in context.
func JWTMiddleware(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return RespondError(c, appErrors.ErrUnauthorized)
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return RespondError(c, appErrors.ErrUnauthorized)
			}

			tokenStr := parts[1]
			claims, err := auth.ParseToken(tokenStr, secret)
			if err != nil {
				return RespondError(c, appErrors.ErrUnauthorized)
			}

			if claims.UserID == "" {
				return RespondError(c, appErrors.ErrUnauthorized)
			}

			c.Set("user_id", claims.UserID)
			return next(c)
		}
	}
}

// GetAuthenticatedUserID retrieves the authenticated user_id from context.
func GetAuthenticatedUserID(c echo.Context) (string, error) {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return "", appErrors.ErrUnauthorized
	}
	return userID, nil
}
