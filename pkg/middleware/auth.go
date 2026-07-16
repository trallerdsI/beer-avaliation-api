package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"beer-review-app/pkg/auth"
	"beer-review-app/pkg/response"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

// UserIDFromContext extrai o user_id injetado pelo middleware Auth.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDContextKey).(string)
	return v, ok
}

// Auth é um middleware de autenticação JWT que envolve um http.HandlerFunc.
// Uso: mux.HandleFunc("GET /api/v1/users/{id}", middleware.Auth(handler))
func Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.SendError(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// strings.Cut evita alocar um slice de partes (zero-alloc no hot path).
		scheme, token, ok := strings.Cut(authHeader, " ")
		if !ok || scheme != "Bearer" || token == "" {
			response.SendError(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		userID, err := auth.ValidateToken(token)
		if err != nil {
			slog.WarnContext(ctx, "invalid token", "err", err)
			response.SendError(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user ID to request context.
		ctx = context.WithValue(ctx, userIDContextKey, userID)
		slog.InfoContext(ctx, "authenticated request", "user_id", userID)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
