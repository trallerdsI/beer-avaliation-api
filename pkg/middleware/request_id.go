package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

const (
	requestIDKey contextKey = "requestID"
	loggerKey    contextKey = "logger"
)

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		// Anexa um logger com o request_id ao contexto para uso nos handlers.
		logger := slog.With(slog.Default(), "request_id", requestID)
		ctx = context.WithValue(ctx, loggerKey, logger)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggerFromContext retorna o logger enriquecido do contexto, ou o default.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// TraceIDFromContext devolve o ID de correlação (request_id) injetado pelo
// RequestIDMiddleware, ou "" se ausente. Usado para popular o campo trace_id
// do Problem RFC 7807, permitindo ao suporte correlacionar erros com os logs.
func TraceIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}
