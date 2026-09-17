package app

import (
	"testing"
)

// TestResolveDBConnStringFromIndividualVars valida a recuperação defensiva:
// quando a string completa não existe, compõe-a a partir das variáveis
// individuais (DB_USER/DB_PASSWORD/DB_HOST/DB_PORT/DB_NAME/DB_SSLMODE).
// Determinístico, sem rede.
func TestResolveDBConnStringFromIndividualVars(t *testing.T) {
	t.Setenv("DB_USER", "beer_user")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "beer_review")
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("DB_CONN_STRING", "")
	t.Setenv("DBConnString", "")

	got := resolveDBConnString()
	want := "postgres://beer_user:secret@localhost:5432/beer_review?sslmode=disable"
	if got != want {
		t.Fatalf("resolveDBConnString() = %q, want %q", got, want)
	}
}

// TestResolveDBConnStringPrefersFull garante que a string completa tem
// prioridade sobre a composição individual.
func TestResolveDBConnStringPrefersFull(t *testing.T) {
	t.Setenv("DB_CONN_STRING", "postgres://full@host:5432/db?sslmode=require")
	t.Setenv("DB_USER", "ignored")

	if got := resolveDBConnString(); got != "postgres://full@host:5432/db?sslmode=require" {
		t.Fatalf("expected full string preference, got %q", got)
	}
}

func TestResolveDBConnStringPrefersPooledPostgresURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_CONN_STRING", "")
	t.Setenv("DBConnString", "")
	t.Setenv("POSTGRES_URL", "postgres://pooled@host:5432/db")
	t.Setenv("POSTGRES_URL_NON_POOLING", "postgres://direct@host:5432/db")

	if got := resolveDBConnString(); got != "postgres://pooled@host:5432/db" {
		t.Fatalf("expected pooled URL preference, got %q", got)
	}
}

func TestNormalizePostgresDSNRemovesUnsupportedChannelBinding(t *testing.T) {
	dsn := "postgresql://user:password@host/db?channel_binding=require&sslmode=require"
	got := normalizePostgresDSN(dsn)
	want := "postgresql://user:password@host/db?sslmode=require"
	if got != want {
		t.Fatalf("normalizePostgresDSN() = %q, want %q", got, want)
	}
}

// TestResolveDBConnStringEmpty confirma que, sem nenhuma variável, devolve "".
func TestResolveDBConnStringEmpty(t *testing.T) {
	t.Setenv("DB_CONN_STRING", "")
	t.Setenv("DBConnString", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_NAME", "")

	if got := resolveDBConnString(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
