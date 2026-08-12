package app

import (
	"bytes"
	"database/sql"
	"embed"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"log/slog"
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

func TestResolveDBConnStringComposesFromParts(t *testing.T) {
	os.Unsetenv("DB_CONN_STRING")
	os.Unsetenv("DBConnString")
	os.Unsetenv("POSTGRES_URL_NON_POOLING")
	os.Unsetenv("POSTGRES_URL")

	t.Setenv("DB_USER", "custom_user")
	t.Setenv("DB_PASSWORD", "p@ss")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_NAME", "app_db")
	t.Setenv("DB_SSLMODE", "require")

	got := resolveDBConnString()
	want := "postgres://custom_user:p%40ss@db.example.com:5433/app_db?sslmode=require"
	if got != want {
		t.Fatalf("composed DSN = %q, want %q", got, want)
	}
}

func TestResolveDBConnStringReturnsEmptyWhenMissingUserOrDB(t *testing.T) {
	os.Unsetenv("DB_CONN_STRING")
	os.Unsetenv("DBConnString")
	os.Unsetenv("POSTGRES_URL_NON_POOLING")
	os.Unsetenv("POSTGRES_URL")
	os.Unsetenv("DB_USER")
	os.Unsetenv("POSTGRES_USER")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("POSTGRES_DATABASE")

	if got := resolveDBConnString(); got != "" {
		t.Fatalf("expected empty DSN when user and db are missing, got %q", got)
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

func TestExecuteSQLFile_ReadsFromEmbed(t *testing.T) {
	data, err := readMigrationSQL("create_beers_table.sql")
	if err != nil {
		t.Fatalf("readMigrationSQL: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty SQL content")
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
		{"postgres://u:p@h:5432/d", "postgres://****@h:5432/d"},
		{"postgres://user@host/db", "postgres://****@host/db"},
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

func TestOpenapiSpecHandlerMissingFile(t *testing.T) {
	orig := embeddedOpenAPI
	defer func() { embeddedOpenAPI = orig }()
	embeddedOpenAPI = embed.FS{}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
	openapiSpecHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when openapi missing, got %d", rr.Code)
	}
}

func TestBuildRouterWithDBErr_NilDB_Returns503ForDataRoutes(t *testing.T) {
	logger := testLogger(t)
	handler := BuildRouterWithDBErr(nil, sql.ErrConnDone, logger)

	dataEndpoints := []string{
		"/api/v1/beers",
		"/api/v1/stats",
	}
	for _, ep := range dataEndpoints {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: expected 503, got %d", ep, rr.Code)
		}
	}
}

func TestBuildRouterWithDBErr_NilDB_OperationalEndpoints(t *testing.T) {
	logger := testLogger(t)
	handler := BuildRouterWithDBErr(nil, sql.ErrConnDone, logger)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("health: expected 200 or 503, got %d", rr.Code)
	}
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}
