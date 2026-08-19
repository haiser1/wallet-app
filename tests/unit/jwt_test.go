package unit

import (
	"testing"

	"test-teknis/internal/auth"
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
