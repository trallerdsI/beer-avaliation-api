package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	appMetrics "beer-review-app/pkg/metrics"
)

func TestResolveDBConnStringPrefersExplicitEnvVars(t *testing.T) {
	t.Setenv("DB_CONN_STRING", "postgres://local")
	t.Setenv("DBConnString", "postgres://vercel")

	if got := resolveDBConnString(); got != "postgres://local" {
		t.Fatalf("expected DB_CONN_STRING to be preferred, got %q", got)
	}
}

func TestResolveDBConnStringFallsBackToVercelStyleEnv(t *testing.T) {
	os.Unsetenv("DB_CONN_STRING")
	t.Setenv("DBConnString", "postgres://vercel")

	if got := resolveDBConnString(); got != "postgres://vercel" {
		t.Fatalf("expected DBConnString fallback, got %q", got)
	}
}

func TestIsServerlessRuntimeDetectsVercelEnv(t *testing.T) {
	t.Setenv("VERCEL", "1")
	if got := appMetrics.IsServerlessRuntime(); !got {
		t.Fatal("expected Vercel environment to be detected as serverless")
	}
}

func TestReadMigrationSQLSupportsEmbeddedFiles(t *testing.T) {
	data, err := readMigrationSQL("migrations/create_beers_table.sql")
	if err != nil {
		t.Fatalf("expected embedded migration to be readable, got error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected embedded migration contents to be non-empty")
	}
	if !bytes.Contains(data, []byte("CREATE TABLE IF NOT EXISTS beers")) {
		t.Fatalf("expected embedded migration to contain beers table creation statement")
	}
}

func TestMaskPassword(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"postgres://user:secret@host:5432/db", "postgres://****@host:5432/db"},
		{"postgres://host:5432/db", "***masked***"},
		{"invalid", "***masked***"},
	}
	for _, tc := range cases {
		if got := maskPassword(tc.input); got != tc.want {
			t.Fatalf("maskPassword(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMaxConnsFromEnv(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "20")
	if got := maxOpenConns(); got != 20 {
		t.Fatalf("expected 20, got %d", got)
	}
	t.Setenv("DB_MAX_OPEN_CONNS", "0")
	if got := maxOpenConns(); got != 10 {
		t.Fatalf("expected default 10, got %d", got)
	}
}

func TestMaxIdleConnsFromEnv(t *testing.T) {
	t.Setenv("DB_MAX_IDLE_CONNS", "3")
	if got := maxIdleConns(); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
	t.Setenv("DB_MAX_IDLE_CONNS", "0")
	if got := maxIdleConns(); got != 5 {
		t.Fatalf("expected default 5, got %d", got)
	}
}

func TestDocsHandlerReturnsHTML(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	docsHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html content type, got %q", ct)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("swagger-ui")) {
		t.Fatalf("expected swagger-ui in body")
	}
}

func TestOpenapiSpecHandlerReturnsYAML(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	openapiSpecHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Fatalf("expected application/yaml, got %q", ct)
	}
	if len(rr.Body.Bytes()) == 0 {
		t.Fatal("expected non-empty openapi body")
	}
}
