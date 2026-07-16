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

// Auth é um middleware de autenticação JWT que envolve um http.HandlerFunc.
// Uso: mux.HandleFunc("GET /api/v1/users/{id}", middleware.Auth(handler))
func Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.SendError(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			response.SendError(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		userID, err := auth.ValidateToken(tokenParts[1])
		if err != nil {
			response.SendError(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user ID to request context.
		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		slog.InfoContext(ctx, "authenticated request", "user_id", userID)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
