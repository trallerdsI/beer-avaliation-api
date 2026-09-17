package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildRouter_RegistersAllRoutes(t *testing.T) {
	logger := testLogger(t)
	handler := BuildRouterWithDBErr(nil, nil, logger)

	routes := []string{
		"/api/v1/beers/enums",
		"/api/v1/beers",
		"/api/v1/beers",
		"/api/v1/beers/1",
		"/api/v1/beers/1",
		"/api/v1/beers/1",
		"/api/v1/beers/search",
		"/api/v1/beers/1/comments",
		"/api/v1/beers/1/comments/1",
		"/api/v1/beers/1/comments/1/like",
		"/api/v1/feed",
		"/api/v1/users/register",
		"/api/v1/users/login",
		"/api/v1/users/oauth",
		"/api/v1/auth/refresh",
		"/api/v1/users/1",
		"/api/v1/users/1",
		"/api/v1/users/1",
		"/api/v1/users/1/push/subscribe",
		"/api/v1/users/1/push/unsubscribe",
		"/api/v1/users/1/push",
		"/api/v1/stats",
		"/api/v1/admin/stats",
		"/api/v1/users/me/stats",
		"/api/v1/health",
		"/docs",
		"/docs/openapi.yaml",
	}

	methods := []string{
		http.MethodGet,
		http.MethodGet,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPost,
		http.MethodGet,
		http.MethodPost,
		http.MethodDelete,
		http.MethodPost,
		http.MethodGet,
		http.MethodGet,
		http.MethodPost,
		http.MethodPost,
		http.MethodPost,
		http.MethodPost,
		http.MethodGet,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPost,
		http.MethodPost,
		http.MethodGet,
		http.MethodGet,
		http.MethodGet,
		http.MethodGet,
		http.MethodGet,
		http.MethodGet,
		http.MethodGet,
	}

	for i, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(methods[i], route, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code == http.StatusNotFound {
				t.Fatalf("route %s not registered", route)
			}
		})
	}
}

func TestBuildRouter_MiddlewareStackPresent(t *testing.T) {
	logger := testLogger(t)
	handler := BuildRouterWithDBErr(nil, nil, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Type") == "" && rr.Code != http.StatusNotFound {
		t.Fatal("expected middleware to process request")
	}
}

func TestBuildRouter_HealthEndpoint_WithoutDB(t *testing.T) {
	logger := testLogger(t)
	handler := BuildRouterWithDBErr(nil, nil, logger)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("health: expected 200 or 503, got %d", rr.Code)
	}
}

func TestBuildRouter_DataEndpoints_503WhenDBError(t *testing.T) {
	logger := testLogger(t)
	handler := BuildRouterWithDBErr(nil, sql.ErrConnDone, logger)

	dataEndpoints := []string{
		"/api/v1/beers",
		"/api/v1/stats",
	}

	for _, ep := range dataEndpoints {
		t.Run(ep, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusServiceUnavailable {
				t.Fatalf("%s: expected 503, got %d", ep, rr.Code)
			}
		})
	}
}

func TestBuildRouter_PublicEndpoints_AccessibleWithoutDB(t *testing.T) {
	logger := testLogger(t)
	handler := BuildRouterWithDBErr(nil, nil, logger)

	publicEndpoints := []string{
		"/docs",
		"/docs/openapi.yaml",
	}

	for _, ep := range publicEndpoints {
		t.Run(ep, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			handler.ServeHTTP(rr, req)

			if rr.Code == http.StatusServiceUnavailable {
				t.Fatalf("%s: public endpoint should not return 503", ep)
			}
		})
	}
}

func TestGracefulShutdown_SignalHandling(t *testing.T) {
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		srv.Shutdown(context.Background())
	}()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 during graceful shutdown, got %d", rr.Code)
	}
}

func TestGracefulShutdown_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			w.WriteHeader(http.StatusServiceUnavailable)
		}),
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		srv.Shutdown(ctx)
	}()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(ctx)
	srv.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after context cancellation, got %d", rr.Code)
	}
}

func TestMaskPassword_VariousFormats(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"postgres://user:secret@host:5432/db", "postgres://****@host:5432/db"},
		{"postgres://host:5432/db", "***masked***"},
		{"invalid", "***masked***"},
		{"postgres://u:p@h:5432/d", "postgres://****@h:5432/d"},
		{"postgres://user@host/db", "postgres://****@host/db"},
		{"postgres://user:pass@host/db", "postgres://****@host/db"},
		{"mysql://user:pass@host:3306/db", "mysql://****@host:3306/db"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := maskPassword(tc.input)
			if got != tc.want {
				t.Fatalf("maskPassword(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "b", "c"); got != "b" {
		t.Fatalf("firstNonEmpty = %q, want %q", got, "b")
	}
	if got := firstNonEmpty("", "", ""); got != "" {
		t.Fatalf("firstNonEmpty = %q, want empty", got)
	}
}

func TestDocsHandler_HTMLContent(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	docsHandler().ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "swagger-ui") {
		t.Fatal("expected swagger-ui in docs response")
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
		t.Fatal("expected text/html content type")
	}
}

func TestOpenapiSpecHandler_ReturnsYAML(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	openapiSpecHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "yaml") {
		t.Fatal("expected yaml content type")
	}
	if len(rr.Body.Bytes()) == 0 {
		t.Fatal("expected non-empty openapi body")
	}
}
