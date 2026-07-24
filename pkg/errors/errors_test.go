package errors

import (
	"errors"
	"testing"
)

func TestAppErrorUnwrapAndIs(t *testing.T) {
	cause := errors.New("db connection refused")
	appErr := NewAppError(500, "failed to create beer", cause)

	// Unwrap expõe a causa para errors.Is.
	if !errors.Is(appErr, cause) {
		t.Fatal("expected errors.Is to match the wrapped cause")
	}

	// errors.As extrai o *AppError da cadeia.
	var got *AppError
	if !errors.As(appErr, &got) {
		t.Fatal("expected errors.As to extract *AppError")
	}
	if got.Code != 500 {
		t.Fatalf("expected code 500, got %d", got.Code)
	}

	// Is casa por Code (AppError.Is).
	target := NewAppError(500, "other message", nil)
	if !errors.Is(appErr, target) {
		t.Fatal("expected errors.Is to match by Code via AppError.Is")
	}

	// AppError com mesmo Code mas causa diferente ainda casa por Code.
	other := NewAppError(500, "x", errors.New("y"))
	if !appErr.Is(other) {
		t.Fatal("expected AppError.Is to match by code")
	}
}

// TestAppErrorError verifies the Error() string formatting with and without a cause.
func TestAppErrorError(t *testing.T) {
	withCause := NewAppError(500, "boom", errors.New("root"))
	if got := withCause.Error(); got != "Error 500: boom: root" {
		t.Fatalf("unexpected Error() with cause: %q", got)
	}

	withoutCause := NewAppError(400, "bad", nil)
	if got := withoutCause.Error(); got != "Error 400: bad" {
		t.Fatalf("unexpected Error() without cause: %q", got)
	}
}

// TestNewAppErrorWithDetail verifies code + detail are stored for client-safe payloads.
func TestNewAppErrorWithDetail(t *testing.T) {
	detail := []map[string]any{{"id": "12"}}
	appErr := NewAppErrorWithDetail(409, "duplicado", "DUPLICATE_BEER", detail)

	if appErr.Code != 409 {
		t.Fatalf("expected 409, got %d", appErr.Code)
	}
	if appErr.ErrorCode != "DUPLICATE_BEER" {
		t.Fatalf("expected DUPLICATE_BEER, got %q", appErr.ErrorCode)
	}
	if appErr.Detail != "duplicado" {
		t.Fatalf("expected detail 'duplicado', got %q", appErr.Detail)
	}
}

// TestNewUnavailableError verifies the 503 fallback error.
func TestNewUnavailableError(t *testing.T) {
	appErr := NewUnavailableError()
	if appErr.Code != 503 {
		t.Fatalf("expected 503, got %d", appErr.Code)
	}
	if !errors.Is(appErr, ErrDatabaseUnavailable) {
		t.Fatal("expected ErrDatabaseUnavailable to be the cause")
	}
}
