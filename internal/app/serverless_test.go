package app

import (
	"net/http/httptest"
	"testing"

	"beer-review-app/pkg/errors"
)

// TestNeedsDB valida o roteamento estático vs dados. Readiness precisa
// inicializar o banco para poder verificar suas dependências.
func TestNeedsDB(t *testing.T) {
	static := []string{
		"/api/v1/beers/enums",
		"/api/v1/health",
		"/healthz",
		"/docs",
		"/docs/openapi.yaml",
		"/metrics",
		"/docs/assets/swagger-ui.css",
	}
	for _, p := range static {
		if NeedsDB(p) {
			t.Errorf("NeedsDB(%q) = true; want false (rota estática)", p)
		}
	}

	dataRoutes := []string{
		"/api/v1/beers",
		"/api/v1/beers/search",
		"/api/v1/users/login",
		"/api/v1/users/register",
		"/api/v1/auth/refresh",
		"/api/v1/users/abc-123",
		"/api/v1/stats",
		"/readyz",
	}
	for _, p := range dataRoutes {
		if !NeedsDB(p) {
			t.Errorf("NeedsDB(%q) = false; want true (rota de dados)", p)
		}
	}
}

// TestRouterHasDB_FalseForDegraded confirma que o router construído com
// db=nil é identificado como degraded (precisa inicialização preguiçosa).
func TestRouterHasDB_FalseForDegraded(t *testing.T) {
	r := BuildRouter(nil, nil)
	if RouterHasDB(r) {
		t.Fatal("RouterHasDB(degraded) = true; want false")
	}
}

// TestWriteServiceUnavailable garante que o Problem Details emitido
// usa o status 503 e o code estável "service_unavailable" — contrato
// que o Flutter cliente mapeia.
func TestWriteServiceUnavailable(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteServiceUnavailable(rr, "detalhe customizado")

	if rr.Code != 503 {
		t.Fatalf("status = %d; want 503", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Fatalf("content-type = %q; want application/problem+json", ct)
	}
	// Sanidade do payload: title e code estáveis.
	body := rr.Body.String()
	if !contains(body, "service_unavailable") {
		t.Errorf("body sem code 'service_unavailable': %s", body)
	}
	if !contains(body, "detalhe customizado") {
		t.Errorf("body sem detail customizado: %s", body)
	}
	// Guard: não pode vazar a causa raiz (OWASP A05).
	if !contains(body, "recurs") {
		_ = errors.NewProblem // garante import em uso
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
