package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/response"
)

const (
	defaultWriteRateLimit       = 10
	defaultWriteRateLimitWindow = time.Minute
	maxRateLimitKeys            = 100000 // hard cap to prevent unbounded memory growth
	rateLimitCleanupInterval    = 5 * time.Minute
)

var rateLimitedTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_rate_limited_total",
		Help: "Total number of requests rejected by rate limit middleware.",
	},
	[]string{"method", "path", "key_type"},
)

func init() {
	prometheus.DefaultRegisterer.Register(rateLimitedTotal)
}

type rateLimiter struct {
	mu           sync.Mutex
	hits         map[string][]time.Time
	limit        int
	window       time.Duration
	lastCleanup  time.Time
}

func newRateLimiter() *rateLimiter {
	return newRateLimiterWith(defaultWriteRateLimit, defaultWriteRateLimitWindow)
}

func newRateLimiterWith(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		hits:        make(map[string][]time.Time),
		limit:       limit,
		window:      window,
		lastCleanup: time.Now(),
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Periodic full cleanup to bound memory (runs at most once per interval)
	if now.Sub(rl.lastCleanup) >= rateLimitCleanupInterval {
		rl.cleanupExpired(now)
		rl.lastCleanup = now
	}

	// Enforce hard cap: evict oldest entry if at capacity
	if len(rl.hits) >= maxRateLimitKeys {
		rl.evictOldest(now)
	}

	times := rl.hits[key]
	filtered := times[:0]
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

func (rl *rateLimiter) cleanupExpired(now time.Time) {
	cutoff := now.Add(-rl.window)
	for k, times := range rl.hits {
		filtered := times[:0]
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
}

func (rl *rateLimiter) evictOldest(now time.Time) {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, times := range rl.hits {
		if len(times) == 0 {
			continue
		}
		t := times[0]
		if first || t.Before(oldestTime) {
			oldestTime = t
			oldestKey = k
			first = false
		}
	}
	if oldestKey != "" {
		delete(rl.hits, oldestKey)
	}
}

func clientKey(r *http.Request) string {
	if uid, ok := UserIDFromContext(r.Context()); ok && uid != "" {
		return "u:" + uid
	}
	ip := remoteIP(r.RemoteAddr)
	if strings.EqualFold(os.Getenv("TRUST_PROXY"), "true") {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			ip = strings.TrimSpace(parts[0])
		}
	}
	return "i:" + ip
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func clientKeyType(key string) string {
	if strings.HasPrefix(key, "u:") {
		return "user"
	}
	return "ip"
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
			rateLimitedTotal.WithLabelValues(r.Method, r.URL.Path, clientKeyType(key)).Inc()
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
	if rateLimitDisabled() {
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	rl := newRateLimiterWith(limit, window)
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
				rateLimitedTotal.WithLabelValues(r.Method, r.URL.Path, clientKeyType(key)).Inc()
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
