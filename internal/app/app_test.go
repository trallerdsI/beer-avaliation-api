package app

import (
	"os"
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
