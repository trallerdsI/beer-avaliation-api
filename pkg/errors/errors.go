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

// Problem implementa o padrão RFC 7807 (Problem Details for HTTP APIs) de forma
// pragmática, para comunicação tipada e segura de erros entre a API (Go) e os
// clientes (ex: Flutter). Regras de segurança/LGPD aplicadas:
//   - Nunca expõe a causa raiz nem dados pessoais (PII) no corpo.
//   - `Message` é genérico/fallback (PT-BR por defeito, art. 48 LGPD).
//   - `Code` é uma chave estável snake_case para o cliente traduzir via i18n
//     localmente, sem depender de strings do backend.
//   - `TraceID` permite ao suporte correlacionar o erro com os logs servidor.
//   - `Details` descreve erros granulares (ex: validação de campos).
//   - `Action` sugere um fluxo à UX (relogin/redirect/retry) sem a forçar.
type Problem struct {
	Type    string           `json:"type"`              // URI relativa do erro (ex: /errors/duplicate_beer)
	Title   string           `json:"title"`             // Resumo legível (http.StatusText)
	Status  int              `json:"status"`            // Cópia do HTTP status
	Code    string           `json:"code,omitempty"`    // Chave i18n estável (snake_case)
	Message string           `json:"message"`           // Mensagem fallback (não expõe causa)
	TraceID string           `json:"trace_id,omitempty"` // ID de correlação (RequestIDMiddleware)
	Details []ProblemDetail  `json:"details,omitempty"`  // Erros granulares (ex: validação)
	Action  *ProblemAction   `json:"action,omitempty"`   // Sugestão de fluxo para a UX
}

// ProblemDetail descreve um erro granular (ex: um campo inválido).
type ProblemDetail struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`    // Código de erro do campo (ex: email_invalid)
	Message string `json:"message"` // Mensagem legível do campo (i18n no cliente)
}

// ProblemAction sugere à UX um fluxo a seguir (sem o obrigar).
type ProblemAction struct {
	Type string            `json:"type"`           // "relogin" | "redirect" | "retry"
	URI  string            `json:"uri,omitempty"`  // Deeplink/URL interna
	Meta map[string]string `json:"meta,omitempty"` // Metadados adicionais (não-PII)
}

// AppError é o erro de domínio da aplicação, transportável até à camada HTTP
// onde é convertido num Problem (RFC 7807). Os campos Code/Details/Action/
// TraceID espelham o Problem, permitindo mapeamento direto e sem perda de
// contexto seguro.
type AppError struct {
	Code      int              // HTTP status sugerido
	Message   string           // Mensagem de erro (não expõe causa raiz)
	Err       error            // Causa interna (apenas para logs servidor)
	ErrorCode string           // Código de erro estável para o cliente (ex: "DUPLICATE_BEER")
	Detail    any              // Payload extra seguro (legado; preferir Details)
	Details   []ProblemDetail  // Erros granulares tipados (ex: validação)
	Action    *ProblemAction   // Sugestão de fluxo para a UX
	TraceID   string           // ID de correlação (RequestIDMiddleware)
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

// NewAppErrorWithCode cria um AppError com código de erro estável (i18n).
func NewAppErrorWithCode(code int, message, errorCode string) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		ErrorCode: errorCode,
	}
}

// NewAppErrorWithDetail cria um AppError com código de erro estável e um
// payload extra seguro (não expõe causa interna) para o cliente.
//
// Deprecated: preferir NewAppErrorWithDetails com Details tipados. Mantido
// para compatibilidade com chamadas existentes (ex: sugestões de duplicado).
func NewAppErrorWithDetail(code int, message, errorCode string, detail any) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		ErrorCode: errorCode,
		Detail:    detail,
	}
}

// NewAppErrorWithDetails cria um AppError com código estável e detalhes
// granulares tipados (RFC 7807), seguro para o cliente (sem PII).
func NewAppErrorWithDetails(code int, message, errorCode string, details []ProblemDetail) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		ErrorCode: errorCode,
		Details:   details,
	}
}

// ToProblem converte o AppError num Problem (RFC 7807), pronto a serializar.
// O type é derivado do ErrorCode (URI relativa) quando disponível.
func (e *AppError) ToProblem() *Problem {
	code := e.ErrorCode
	if code == "" {
		code = HTTPStatusSlug(e.Code)
	}
	p := &Problem{
		Type:    "/errors/" + code,
		Title:   statusText(e.Code),
		Status:  e.Code,
		Code:    code,
		Message: e.Message,
		TraceID: e.TraceID,
		Action:  e.Action,
	}
	if len(e.Details) > 0 {
		p.Details = e.Details
	} else if e.Detail != nil {
		// Legado: detalhe livre mantido em Details como item genérico.
		p.Details = []ProblemDetail{{Code: code, Message: fmt.Sprintf("%v", e.Detail)}}
	}
	return p
}
