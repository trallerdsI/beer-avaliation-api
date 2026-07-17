package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"beer-review-app/internal/user/model"
	"beer-review-app/pkg/auth"
	"beer-review-app/pkg/response"
)

type contextKey string

const (
	userIDContextKey contextKey = "user_id"
	roleContextKey   contextKey = "role"
)

// UserIDFromContext extrai o user_id injetado pelo middleware Auth.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDContextKey).(string)
	return v, ok
}

// RoleFromContext extrai o role (user/admin) injetado pelo middleware Auth.
func RoleFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(roleContextKey).(string)
	return v, ok
}

// IsAdmin devolve true se o contexto tiver role de administrador.
func IsAdmin(ctx context.Context) bool {
	role, ok := RoleFromContext(ctx)
	return ok && role == model.RoleAdmin
}

// Auth é um middleware de autenticação JWT que envolve um http.HandlerFunc.
// Injeta user_id e role no contexto da requisição.
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

		userID, role, err := auth.ValidateToken(token)
		if err != nil {
			slog.WarnContext(ctx, "invalid token", "err", err)
			response.SendError(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx = context.WithValue(ctx, userIDContextKey, userID)
		ctx = context.WithValue(ctx, roleContextKey, role)
		slog.InfoContext(ctx, "authenticated request", "user_id", userID, "role", role)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// RequireAdmin encapsula Auth e exige papel de administrador; caso contrário
// responde 403. Usado em rotas de gestão global (ex: listar/apagar utilizadores).
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return Auth(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdmin(r.Context()) {
			response.SendError(w, "admin privileges required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
