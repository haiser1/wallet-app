package unit

import (
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/haiser1/wallet-app/internal/auth"
	"github.com/haiser1/wallet-app/internal/middleware"
)

func TestJWT_GenerateAndParseToken(t *testing.T) {
	secret := "test-secret-key"
	userID := "11111111-1111-1111-1111-111111111111"

	tokenStr, err := auth.GenerateToken(userID, secret)
	if err != nil {
		t.Fatalf("GenerateToken error = %v", err)
	}

	if tokenStr == "" {
		t.Fatal("expected non-empty token string")
	}

	claims, err := auth.ParseToken(tokenStr, secret)
	if err != nil {
		t.Fatalf("ParseToken error = %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected UserID %s, got %s", userID, claims.UserID)
	}
}

func TestJWT_InvalidSecret(t *testing.T) {
	secret := "correct-secret"
	wrongSecret := "wrong-secret"
	userID := "11111111-1111-1111-1111-111111111111"

	tokenStr, _ := auth.GenerateToken(userID, secret)

	_, err := auth.ParseToken(tokenStr, wrongSecret)
	if err == nil {
		t.Fatal("expected error when parsing token with wrong secret")
	}
}

func TestJWT_MalformedToken(t *testing.T) {
	secret := "test-secret"
	_, err := auth.ParseToken("not.a.valid.jwt.token", secret)
	if err == nil {
		t.Fatal("expected error when parsing malformed token")
	}
}

func TestJWTMiddleware_BearerAndRawToken(t *testing.T) {
	secret := "test-secret"
	userID := "11111111-1111-1111-1111-111111111111"
	tokenStr, _ := auth.GenerateToken(userID, secret)

	e := echo.New()
	mw := middleware.JWTMiddleware(secret)

	h := mw(func(c echo.Context) error {
		gotID, err := middleware.GetAuthenticatedUserID(c)
		if err != nil {
			return err
		}
		if gotID != userID {
			t.Errorf("expected %s, got %s", userID, gotID)
		}
		return c.String(200, "ok")
	})

	// Case 1: Bearer prefix
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("Authorization", "Bearer "+tokenStr)
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	if err := h(c1); err != nil {
		t.Fatalf("Bearer token failed: %v", err)
	}

	// Case 2: Raw token (Swagger UI direct header format)
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Authorization", tokenStr)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	if err := h(c2); err != nil {
		t.Fatalf("Raw token failed: %v", err)
	}
}
