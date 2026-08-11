package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestCORSMiddlewareAllowsOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()

	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Fatalf("expected Access-Control-Allow-Origin to be set, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddlewareBlocksOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers", nil)
	req.Header.Set("Origin", "https://evil.com")
	rr := httptest.NewRecorder()

	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestCORSMiddlewareSkipsWithoutOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers", nil)
	rr := httptest.NewRecorder()

	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no CORS headers, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddlewareOptionsPreflight(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com")
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/beers", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()

	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("expected Access-Control-Allow-Methods to be set")
	}
}

func TestParseList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", nil},
		{"single", "https://example.com", []string{"https://example.com"}},
		{"multiple", "https://a.com, https://b.com", []string{"https://a.com", "https://b.com"}},
		{"with spaces", "  https://a.com  ,  https://b.com  ", []string{"https://a.com", "https://b.com"}},
		{"with empty", "https://a.com,,https://b.com", []string{"https://a.com", "https://b.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseList(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("parseList(%q) len = %d, want %d", tt.input, len(got), len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestCompileRegex(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"empty pattern", "", false},
		{"valid pattern", "^https://.*\\.example\\.com$", true},
		{"invalid pattern", "[invalid", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := compileRegex(tt.pattern)
			if tt.expected {
				if re == nil {
					t.Fatalf("compileRegex(%q) = nil, want non-nil", tt.pattern)
				}
			} else {
				if re != nil {
					t.Fatalf("compileRegex(%q) = %v, want nil", tt.pattern, re)
				}
			}
		})
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		allowed  []string
		re       *string
		expected bool
	}{
		{"exact match", "https://example.com", []string{"https://example.com"}, nil, true},
		{"no match", "https://evil.com", []string{"https://example.com"}, nil, false},
		{"regex match", "https://sub.example.com", []string{}, strPtr("^https://.*\\.example\\.com$"), true},
		{"regex no match", "https://evil.com", []string{}, strPtr("^https://.*\\.example\\.com$"), false},
		{"empty allowed", "https://example.com", nil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var re *regexp.Regexp
			if tt.re != nil {
				re = compileRegex(*tt.re)
			}
			got := isAllowedOrigin(tt.origin, tt.allowed, re)
			if got != tt.expected {
				t.Errorf("isAllowedOrigin(%q, %v, %v) = %v, want %v", tt.origin, tt.allowed, re, got, tt.expected)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
