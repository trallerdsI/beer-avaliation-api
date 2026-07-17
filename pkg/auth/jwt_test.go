package auth

import (
	stderrors "errors"
	"testing"
	"testing/synctest"
	"time"

	"beer-review-app/pkg/errors"
)

func TestGenerateValidateRoundTrip(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key")
	// Recarrega o segredo para o teste (loadJWTSecret corre no init do package).
	jwtSecret = loadJWTSecret()

	token, err := GenerateToken("user-123", "")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	userID, _, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if userID != "user-123" {
		t.Fatalf("expected user-123, got %q", userID)
	}
}

func TestValidateTokenRejectsTampered(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key")
	jwtSecret = loadJWTSecret()

	token, _ := GenerateToken("user-123", "")
	// Alterar a assinatura deve falhar na verificação HMAC.
	tampered := token[:len(token)-2] + "xx"
	if _, _, err := ValidateToken(tampered); err == nil {
		t.Fatal("expected validation failure for tampered token")
	}
}

func TestValidateTokenExpired(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key")
	jwtSecret = loadJWTSecret()

	// Usa testing/synctest para avançar o relógio virtual até a expiração
	// (24h) de forma determinística, sem time.Sleep real.
	synctest.Test(t, func(t *testing.T) {
		token, err := GenerateToken("user-123", "")
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}
		time.Sleep(tokenTTL + time.Minute)
		if _, _, err := ValidateToken(token); err == nil {
			t.Fatal("expected expired token to be rejected")
		}
	})
}

func TestMissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	jwtSecret = nil

	if _, err := GenerateToken("u", ""); !stderrors.Is(err, errors.ErrMissingSecret) {
		t.Fatalf("expected ErrMissingSecret, got %v", err)
	}
	if _, _, err := ValidateToken("x.y.z"); !stderrors.Is(err, errors.ErrMissingSecret) {
		t.Fatalf("expected ErrMissingSecret, got %v", err)
	}
}
