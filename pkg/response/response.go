package response

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

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

// NewProblem é um helper para construir um Problem mínimo e seguro (PT-BR).
func NewProblem(status int, code, message string) *errors.Problem {
	return &errors.Problem{
		Type:    "/errors/" + code,
		Title:   http.StatusText(status),
		Status:  status,
		Code:    code,
		Message: message,
	}
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
	p := NewProblem(statusCode, errors.HTTPStatusSlug(statusCode), message)
	for _, o := range opts {
		o(p)
	}
	SendProblem(w, p)
}

// SelectFields projeta apenas os campos solicitados de cada elemento de uma
// fatia, reduzindo o tamanho do JSON enviado ao cliente móvel em telas de
// listagem (ex: evita trafegar descrições longas). Se fields estiver vazio,
// retorna o payload original (sem alocação extra de projeção).
//
// Uso: /api/v1/beers?fields=id,name,image_url
//
// Defesa (Pilar 4): limita o número de campos pedidos para evitar abuse de
// CPU/reflexão e cardinalidade de projeção (cap de 32 campos).
func SelectFields(payload any, fields string) any {
	fields = strings.TrimSpace(fields)
	if fields == "" {
		return payload
	}

	const maxFields = 32
	want := make(map[string]struct{}, 8)
	for _, f := range strings.Split(fields, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		if len(want) >= maxFields {
			break
		}
		want[strings.ToLower(f)] = struct{}{}
	}
	if len(want) == 0 {
		return payload
	}

	rv := reflect.ValueOf(payload)
	if rv.Kind() != reflect.Slice {
		return payload
	}

	out := make([]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if elem.Kind() != reflect.Struct {
			// Elemento não-estrutura: mantém como está.
			out = append(out, elem.Interface())
			continue
		}
		proj := make(map[string]any, len(want))
		t := elem.Type()
		for j := 0; j < t.NumField(); j++ {
			f := t.Field(j)
			// Respeita o nome JSON (ou o nome do campo) para casar com o cliente.
			name := jsonName(f)
			if _, ok := want[strings.ToLower(name)]; ok {
				proj[name] = elem.Field(j).Interface()
			}
		}
		out = append(out, proj)
	}
	return out
}

// jsonName extrai o nome do campo conforme a tag `json`, sem opções de omit.
func jsonName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name
	}
	if idx := strings.IndexByte(tag, ','); idx >= 0 {
		tag = tag[:idx]
	}
	if tag == "" {
		return f.Name
	}
	return tag
}
