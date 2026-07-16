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
	if e.Err != nil {
		return fmt.Sprintf("Error %d: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("Error %d: %s", e.Code, e.Message)
}

// Unwrap expõe o erro interno para errors.As/errors.Is (tratamento
// hierárquico idiomático do Go 1.26).
func (e *AppError) Unwrap() error {
	return e.Err
}

// Is satisfaz errors.Is para AppError com o mesmo Code.
func (e *AppError) Is(target error) bool {
	var t *AppError
	if errors.As(target, &t) {
		return t.Code == e.Code
	}
	return false
}

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
