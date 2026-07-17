package errors

import (
	"errors"
	"fmt"
)

// ErrMissingSecret é retornado quando a chave JWT não está configurada.
var ErrMissingSecret = errors.New("jwt secret is not configured")

// ErrDatabaseUnavailable é retornado quando o repositório não consegue ligar
// à base de dados (ex: DB ausente no arranque). Permite às rotas responderem
// 503 de forma determinística em vez de derrubar o processo.
var ErrDatabaseUnavailable = errors.New("database unavailable")

// NewUnavailableError devolve um AppError 503 para o caso de DB ausente.
func NewUnavailableError() *AppError {
	return NewAppError(503, "database is not ready", ErrDatabaseUnavailable)
}

type AppError struct {
	Code      int
	Message   string
	Err       error
	ErrorCode string // código de erro estável para o cliente (ex: "DUPLICATE_BEER")
	Detail    any    // payload extra seguro para o cliente (ex: sugestões)
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

// NewAppErrorWithDetail cria um AppError com código de erro estável e um
// payload extra seguro (não expõe causa interna) para o cliente.
func NewAppErrorWithDetail(code int, message, errorCode string, detail any) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		ErrorCode: errorCode,
		Detail:    detail,
	}
}
