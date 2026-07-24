package response

import (
	"encoding/json"
	"net/http"

	"beer-review-app/pkg/errors"
)

// SendResponse escreve payload JSON com Content-Type apropriado.
// A compressão (gzip/brotli) é aplicada pelo CompressionMiddleware.
func SendResponse(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

// SendProblem escreve um erro no formato RFC 7807 (Problem Details) com o
// Content-Type `application/problem+json`. Nunca expõe a causa interna nem
// dados pessoais — apenas o que o Problem carrega (segurança: OWASP A05, LGPD).
func SendProblem(w http.ResponseWriter, p *errors.Problem) {
	if p == nil {
		p = errors.NewProblem(http.StatusInternalServerError, "internal_server_error", "Erro interno do servidor.")
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// ProblemOption configura campos do Problem (functional options, zero-alloc
// quando não há opções).
type ProblemOption func(*errors.Problem)

// WithProblemCode define a chave i18n estável (ex: "duplicate_beer").
func WithProblemCode(code string) ProblemOption {
	return func(p *errors.Problem) { p.Code = code }
}

// WithProblemTrace associa o ID de correlação (RequestIDMiddleware) para o suporte.
func WithProblemTrace(traceID string) ProblemOption {
	return func(p *errors.Problem) { p.TraceID = traceID }
}

// WithProblemDetails anexa erros granulares tipados (ex: validação de campos).
func WithProblemDetails(details ...errors.ProblemDetail) ProblemOption {
	return func(p *errors.Problem) { p.Details = append(p.Details, details...) }
}

// WithProblemAction sugere um fluxo à UX (relogin/redirect/retry).
func WithProblemAction(action *errors.ProblemAction) ProblemOption {
	return func(p *errors.Problem) { p.Action = action }
}

// SendError escreve um erro RFC 7807 mínimo (apenas message + code opcional),
// mantido para chamadas simples. Para erros de domínio ricos, usar SendProblem
// com um AppError.ToProblem().
func SendError(w http.ResponseWriter, message string, statusCode int, opts ...ProblemOption) {
	p := errors.NewProblem(statusCode, errors.HTTPStatusSlug(statusCode), message)
	for _, o := range opts {
		o(p)
	}
	SendProblem(w, p)
}
