package middleware

import (
	"net/http"
	"os"
	"regexp"
	"strings"
)

// CORSMiddleware implementa CORS production-ready para a API.
// Não usa wildcard "*" com credenciais; valida origins por allowlist e regex.
// Preflight é cacheado por 12h (43200s) para reduzir latência no cliente.
func CORSMiddleware(next http.Handler) http.Handler {
	allowedOrigins := parseList(getEnv("CORS_ALLOWED_ORIGINS", ""))
	allowedRegex := compileRegex(getEnv("CORS_ALLOWED_REGEX", ""))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Sem Origin: requisição nativa (Flutter mobile, curl, Postman). Segue direto.
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !isAllowedOrigin(origin, allowedOrigins, allowedRegex) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, X-CSRF-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "43200")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func compileRegex(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re
}

func isAllowedOrigin(origin string, allowed []string, re *regexp.Regexp) bool {
	for _, o := range allowed {
		if o == origin {
			return true
		}
	}
	if re != nil && re.MatchString(origin) {
		return true
	}
	return false
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
