package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
)

const (
	requestIDKey contextKey = "requestID"
	loggerKey    contextKey = "logger"
)

// newRequestID gera um ID de correlação (16 bytes aleatórios) em stdlib,
// sem depender de github.com/google/uuid (redundante com pkg/uuid).
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "rid-fallback"
	}
	return hex.EncodeToString(b[:])
}

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
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
