package errors

import (
	"errors"
	"testing"
)

func TestAppErrorUnwrapAndIs(t *testing.T) {
	cause := errors.New("db connection refused")
	appErr := NewAppError(500, "failed to create beer", cause)

	if !errors.Is(appErr, cause) {
		t.Fatal("expected errors.Is to match the wrapped cause")
	}

	var got *AppError
	if !errors.As(appErr, &got) {
		t.Fatal("expected errors.As to extract *AppError")
	}
	if got.Code != 500 {
		t.Fatalf("expected code 500, got %d", got.Code)
	}

	target := NewAppError(500, "other message", nil)
	if !errors.Is(appErr, target) {
		t.Fatal("expected errors.Is to match by Code via AppError.Is")
	}

	other := NewAppError(500, "x", errors.New("y"))
	if !appErr.Is(other) {
		t.Fatal("expected AppError.Is to match by code")
	}
}

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

func TestNewUnavailableError(t *testing.T) {
	appErr := NewUnavailableError()
	if appErr.Code != 503 {
		t.Fatalf("expected 503, got %d", appErr.Code)
	}
	if !errors.Is(appErr, ErrDatabaseUnavailable) {
		t.Fatal("expected ErrDatabaseUnavailable to be the cause")
	}
}

func TestNewAppErrorWithCode(t *testing.T) {
	appErr := NewAppErrorWithCode(409, "duplicado", "DUPLICATE_BEER")
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

func TestNewAppErrorWithDetails(t *testing.T) {
	details := []ProblemDetail{{Field: "name", Code: "too_long", Detail: "max 50 chars"}}
	appErr := NewAppErrorWithDetails(400, "validation failed", "VALIDATION_ERROR", details)
	if appErr.Code != 400 {
		t.Fatalf("expected 400, got %d", appErr.Code)
	}
	if len(appErr.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(appErr.Details))
	}
	if appErr.Details[0].Code != "too_long" {
		t.Fatalf("expected detail code 'too_long', got %q", appErr.Details[0].Code)
	}
}

func TestToProblem(t *testing.T) {
	appErr := NewAppErrorWithCode(404, "beer not found", "BEER_NOT_FOUND")
	p := appErr.ToProblem("/api/v1/beers/99")
	if p.Status != 404 {
		t.Fatalf("expected status 404, got %d", p.Status)
	}
	if p.Code != "BEER_NOT_FOUND" {
		t.Fatalf("expected code BEER_NOT_FOUND, got %q", p.Code)
	}
	if p.Instance != "/api/v1/beers/99" {
		t.Fatalf("expected instance /api/v1/beers/99, got %q", p.Instance)
	}
	if p.Type == "" {
		t.Fatal("expected non-empty type")
	}
	if p.Title == "" {
		t.Fatal("expected non-empty title")
	}

	appErr2 := NewAppError(500, "boom", nil)
	p2 := appErr2.ToProblem("/api/v1/x")
	if p2.Code != "internal_server_error" {
		t.Fatalf("expected fallback code internal_server_error, got %q", p2.Code)
	}
}

