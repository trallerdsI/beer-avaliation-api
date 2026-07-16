package errors

import (
	"errors"
	"fmt"
)

// ErrMissingSecret é retornado quando a chave JWT não está configurada.
var ErrMissingSecret = errors.New("jwt secret is not configured")

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("Error %d: %s", e.Code, e.Message)
}

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}