package errors

import (
	"net/http"
	"os"
	"strings"
)

var problemTypeBase = getProblemTypeBase()

func getProblemTypeBase() string {
	base := os.Getenv("PROBLEM_TYPE_BASE")
	if base == "" {
		return "/errors"
	}
	return strings.TrimRight(base, "/")
}

func ProblemType(code string) string {
	return problemTypeBase + "/" + code
}

func SetProblemTypeBase(base string) {
	problemTypeBase = strings.TrimRight(base, "/")
}

// HTTPStatusSlug devolve um slug estável em snake_case para um HTTP status,
// usado como fallback de `code`/`type` quando o AppError não define ErrorCode.
// Ex: 404 -> "not_found", 500 -> "internal_server_error".
func HTTPStatusSlug(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusInternalServerError:
		return "internal_server_error"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "error"
	}
}

// statusText devolve o texto padrão do HTTP status (ex: "Not Found").
func statusText(status int) string {
	return http.StatusText(status)
}

// NewProblem constrói um Problem mínimo e seguro (PT-BR) a partir de um
// status HTTP, código estável e mensagem de fallback.
func NewProblem(status int, code, detail string) *Problem {
	return &Problem{
		Type:   ProblemType(code),
		Title:  statusText(status),
		Status: status,
		Code:   code,
		Detail: detail,
	}
}
