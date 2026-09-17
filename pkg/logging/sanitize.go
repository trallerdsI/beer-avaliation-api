package logging

import (
	"log/slog"
	"strings"
)

// sanitizeReplaceAttr redige valores de chaves sensíveis nos logs.
// Usa slog.ReplaceAttr para interceptar atributos antes da serialização JSON.
func sanitizeReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == "" {
		return a
	}

	lower := strings.ToLower(a.Key)
	switch {
	case lower == "authorization", lower == "token", lower == "password",
		lower == "open_ai_key", lower == "credit_card",
		lower == "db_password", lower == "db_conn_string",
		lower == "supabase_service_role_key", lower == "jwt_secret",
		lower == "api_key", lower == "apikey",
		strings.HasSuffix(lower, "_secret"), strings.HasSuffix(lower, "_key"):
		if a.Value.String() != "" {
			a.Value = slog.StringValue("[REDACTED]")
		}
	}
	return a
}

// SanitizeOptions retorna HandlerOptions com ReplaceAttr configurado para redigir
// valores de chaves sensíveis nos logs (authorization, token, password, *_secret,
// *_key, db_conn_string, jwt_secret, etc.).
func SanitizeOptions(base *slog.HandlerOptions) *slog.HandlerOptions {
	if base == nil {
		base = &slog.HandlerOptions{}
	}
	base.ReplaceAttr = sanitizeReplaceAttr
	return base
}
