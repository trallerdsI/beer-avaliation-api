package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func resetWriteLimiter() {
	writeLimiter = newRateLimiter()
}

func TestIsWriteMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", false},
		{"HEAD", false},
		{"OPTIONS", false},
		{"POST", true},
		{"PUT", true},
		{"DELETE", true},
		{"PATCH", true},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := isWriteMethod(tt.method); got != tt.want {
				t.Errorf("isWriteMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestNewRateLimitMiddleware_UsesCustomLimit(t *testing.T) {
	resetWriteLimiter()
	os.Setenv("RATE_LIMIT_DISABLED", "false")
	defer os.Unsetenv("RATE_LIMIT_DISABLED")

	handler := NewRateLimitMiddleware(2, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", strings.NewReader(`{"name":"IPA"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:12345"
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("request %d: expected 201, got %d", i, rr.Code)
		}
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", strings.NewReader(`{"name":"IPA"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after custom limit, got %d", rr.Code)
	}
}

func TestNewRateLimitMiddleware_DisabledByEnv(t *testing.T) {
	os.Setenv("RATE_LIMIT_DISABLED", "true")
	defer os.Unsetenv("RATE_LIMIT_DISABLED")

	handler := NewRateLimitMiddleware(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", strings.NewReader(`{"name":"IPA"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:12345"
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("request %d: expected 201 when disabled, got %d", i, rr.Code)
		}
	}
}

func TestNewRateLimitMiddleware_SkipsGetRequests(t *testing.T) {
	os.Setenv("RATE_LIMIT_DISABLED", "false")
	defer os.Unsetenv("RATE_LIMIT_DISABLED")

	handler := NewRateLimitMiddleware(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/beers", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 for GET, got %d", i, rr.Code)
		}
	}
}

func TestNewRateLimitMiddleware_InvalidLimit_UsesDefault(t *testing.T) {
	resetWriteLimiter()
	os.Setenv("RATE_LIMIT_DISABLED", "false")
	defer os.Unsetenv("RATE_LIMIT_DISABLED")

	handler := NewRateLimitMiddleware(0, 0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", strings.NewReader(`{"name":"IPA"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 with default limit, got %d", rr.Code)
	}
}

func TestRateLimitMiddlewareAllowsWriteUnderLimit(t *testing.T) {
	resetWriteLimiter()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", strings.NewReader(`{"name":"IPA"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"

	handler := RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestRateLimitMiddlewareRejectsWriteOverLimit(t *testing.T) {
	resetWriteLimiter()
	for i := 0; i < defaultWriteRateLimit; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", strings.NewReader(`{"name":"IPA"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:12345"

		handler := RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("request %d: expected 201, got %d", i, rr.Code)
		}
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", strings.NewReader(`{"name":"IPA"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"

	handler := RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after %d requests, got %d", defaultWriteRateLimit, rr.Code)
	}

	if rr.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected Retry-After 60, got %q", rr.Header().Get("Retry-After"))
	}
}

func TestRateLimitMiddlewareSkipsGetRequests(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	handler := RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET, got %d", rr.Code)
	}
}

func TestRateLimitMiddlewareSkipsRequestsWithoutOrigin(t *testing.T) {
	resetWriteLimiter()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", strings.NewReader(`{"name":"IPA"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"

	handler := RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for request without Origin, got %d", rr.Code)
	}
}

func TestRateLimitMiddlewareSkipsOptionsRequests(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/beers", nil)
	req.Header.Set("Origin", "https://example.com")

	handler := RateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", rr.Code)
	}
}

func TestClientKeyUsesIPWithoutPortAndIgnoresForwardedHeaderByDefault(t *testing.T) {
	t.Setenv("TRUST_PROXY", "false")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.20")

	if got := clientKey(req); got != "i:192.0.2.10" {
		t.Fatalf("clientKey() = %q, want %q", got, "i:192.0.2.10")
	}
}
