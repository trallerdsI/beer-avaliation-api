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
