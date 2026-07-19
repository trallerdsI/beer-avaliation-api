package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	appMetrics "beer-review-app/pkg/metrics"
)

func TestResolveDBConnStringPrefersExplicitEnvVars(t *testing.T) {
	t.Setenv("DB_CONN_STRING", "postgres://local")
	t.Setenv("DBConnString", "postgres://vercel")

	if got := resolveDBConnString(); got != "postgres://local?default_query_exec_mode=simple_protocol&sslmode=require" {
		t.Fatalf("expected DB_CONN_STRING to be preferred, got %q", got)
	}
}

func TestResolveDBConnStringFallsBackToVercelStyleEnv(t *testing.T) {
	os.Unsetenv("DB_CONN_STRING")
	t.Setenv("DBConnString", "postgres://vercel")

	if got := resolveDBConnString(); got != "postgres://vercel?default_query_exec_mode=simple_protocol&sslmode=require" {
		t.Fatalf("expected DBConnString fallback, got %q", got)
	}
}

func TestIsServerlessRuntimeDetectsVercelEnv(t *testing.T) {
	t.Setenv("VERCEL", "1")
	if got := appMetrics.IsServerlessRuntime(); !got {
		t.Fatal("expected Vercel environment to be detected as serverless")
	}
}

func TestResolveMigrationPathUsesRepositoryRoot(t *testing.T) {
	path, err := resolveMigrationPath("migrations/create_beers_table.sql")
	if err != nil {
		t.Fatalf("expected migration path to resolve, got error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected resolved migration file to exist, got error: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("expected migration path to point to a file, got directory")
	}
	if filepath.Base(path) != "create_beers_table.sql" {
		t.Fatalf("expected beers migration filename, got %s", filepath.Base(path))
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
