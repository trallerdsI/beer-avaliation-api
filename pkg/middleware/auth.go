package middleware

import (
	"context"
	"net/http"
	"strings"

	"beer-review-app/pkg/auth"
	"beer-review-app/pkg/response"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		// Add user ID to request context
		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
