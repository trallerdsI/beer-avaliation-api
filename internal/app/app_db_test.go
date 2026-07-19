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
	// Garantir que as chaves de string completa estão ausentes.
	t.Setenv("DB_CONN_STRING", "")
	t.Setenv("DBConnString", "")

	got := resolveDBConnString()
	want := "postgres://beer_user:secret@localhost:5432/beer_review?default_query_exec_mode=simple_protocol&sslmode=require"
	if got != want {
		t.Fatalf("resolveDBConnString() = %q, want %q", got, want)
	}
}

// TestResolveDBConnStringPrefersFull garante que a string completa tem
// prioridade sobre a composição individual.
func TestResolveDBConnStringPrefersFull(t *testing.T) {
	t.Setenv("DB_CONN_STRING", "postgres://full@host:5432/db?sslmode=require")
	t.Setenv("DB_USER", "ignored")

	if got := resolveDBConnString(); got != "postgres://full@host:5432/db?default_query_exec_mode=simple_protocol&sslmode=require" {
		t.Fatalf("expected full string preference, got %q", got)
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

// TestNormalizeSupabaseDSNAddsParams valida que a normalização injeta
// sslmode=require e o protocolo simples do lib/pq (necessário ao PgBouncer),
// preservando o host original quando este já é IPv4-reachável (ex.: localhost).
func TestNormalizeSupabaseDSNAddsParams(t *testing.T) {
	got := normalizeSupabaseDSN("postgres://u:p@localhost:5432/db")
	want := "postgres://u:p@localhost:5432/db?default_query_exec_mode=simple_protocol&sslmode=require"
	if got != want {
		t.Fatalf("normalizeSupabaseDSN() = %q, want %q", got, want)
	}
}

// TestPoolerHostForPrefersEnvPooler valida que a derivação do pooler IPv4
// prioriza o host do pooler já presente em POSTGRES_URL_NON_POOLING.
func TestPoolerHostForPrefersEnvPooler(t *testing.T) {
	t.Setenv("POSTGRES_URL_NON_POOLING", "postgres://u:p@aws-7-saopaulo.pooler.supabase.co:5432/db")
	if got := poolerHostFor("db.abc123.supabase.co"); got != "aws-7-saopaulo.pooler.supabase.co" {
		t.Fatalf("poolerHostFor() = %q, want aws-7-saopaulo.pooler.supabase.co", got)
	}
}
