package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"beer-review-app/pkg/response"
)

const (
	writeRateLimit       = 10
	writeRateLimitWindow = time.Minute
)

type rateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	window  time.Duration
	limit   int
	cleanup time.Duration
	last    time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		hits:    make(map[string][]time.Time),
		limit:   limit,
		window:  window,
		cleanup: window * 2,
	}
	go rl.gc()
	return rl
}

func (rl *rateLimiter) gc() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.cleanup)
		for key, times := range rl.hits {
			filtered := make([]time.Time, 0, len(times))
			for _, t := range times {
				if t.After(cutoff) {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) == 0 {
				delete(rl.hits, key)
			} else {
				rl.hits[key] = filtered
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
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

var writeLimiter = newRateLimiter(writeRateLimit, writeRateLimitWindow)

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	}
	return false
}

func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isWriteMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		key := clientKey(r)
		if !writeLimiter.allow(key) {
			w.Header().Set("Retry-After", "60")
			response.SendProblem(w, response.NewProblem(http.StatusTooManyRequests, "rate_limit_exceeded",
				"Muitas requisições. Tente novamente dentro de 60 segundos."))
			slog.WarnContext(r.Context(), "rate limit exceeded", "key", key, "method", r.Method, "path", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}
