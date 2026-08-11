package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	for i := 0; i < writeRateLimit; i++ {
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
		t.Fatalf("expected 429 after %d requests, got %d", writeRateLimit, rr.Code)
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
