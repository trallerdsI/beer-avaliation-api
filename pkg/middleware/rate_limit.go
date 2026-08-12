package middleware

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/response"
)

const (
	defaultWriteRateLimit       = 10
	defaultWriteRateLimitWindow = time.Minute
)

type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newRateLimiter() *rateLimiter {
	return newRateLimiterWith(defaultWriteRateLimit, defaultWriteRateLimitWindow)
}

func newRateLimiterWith(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	for k, times := range rl.hits {
		filtered := make([]time.Time, 0, len(times))
		for _, t := range times {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			delete(rl.hits, k)
		} else {
			rl.hits[k] = filtered
		}
	}

	times := rl.hits[key]
	filtered := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) >= rl.limit {
		rl.hits[key] = filtered
		return false
	}
	rl.hits[key] = append(filtered, now)
	return true
}

func clientKey(r *http.Request) string {
	if uid, ok := UserIDFromContext(r.Context()); ok && uid != "" {
		return "u:" + uid
	}
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		ip = strings.TrimSpace(parts[0])
	}
	return "i:" + ip
}

var writeLimiter = newRateLimiter()

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	}
	return false
}

func rateLimitDisabled() bool {
	return strings.ToLower(os.Getenv("RATE_LIMIT_DISABLED")) == "true"
}

func RateLimitMiddleware(next http.Handler) http.Handler {
	if rateLimitDisabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isWriteMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		key := clientKey(r)
		if !writeLimiter.allow(key) {
			w.Header().Set("Retry-After", "60")
			response.SendProblem(w, errors.NewProblem(http.StatusTooManyRequests, "rate_limit_exceeded",
				"Muitas requisições. Tente novamente dentro de 60 segundos."))
			slog.WarnContext(r.Context(), "rate limit exceeded", "key", key, "method", r.Method, "path", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func NewRateLimitMiddleware(limit int, window time.Duration) func(http.Handler) http.Handler {
	if limit <= 0 {
		limit = defaultWriteRateLimit
	}
	if window <= 0 {
		window = defaultWriteRateLimitWindow
	}
	rl := newRateLimiterWith(limit, window)
	if rateLimitDisabled() {
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isWriteMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			key := clientKey(r)
			if !rl.allow(key) {
				w.Header().Set("Retry-After", "60")
				response.SendProblem(w, errors.NewProblem(http.StatusTooManyRequests, "rate_limit_exceeded",
					"Muitas requisições. Tente novamente dentro de 60 segundos."))
				slog.WarnContext(r.Context(), "rate limit exceeded", "key", key, "method", r.Method, "path", r.URL.Path)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
