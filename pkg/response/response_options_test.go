package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"beer-review-app/pkg/errors"
)

// TestSendProblemRFC7807 valida o envelope RFC 7807: Content-Type
// application/problem+json, campos type/title/status/code/message e ausência
// de vazamento de causa interna.
func TestSendProblemRFC7807(t *testing.T) {
	rr := httptest.NewRecorder()
	p := errors.NewProblem(http.StatusUnauthorized, "unauthorized", "O cabeçalho de autorização é obrigatório.")
	SendProblem(rr, p)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", ct)
	}

	var body errors.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != "unauthorized" {
		t.Fatalf("expected code unauthorized, got %q", body.Code)
	}
	if body.Status != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", body.Status)
	}
	if body.Title != http.StatusText(http.StatusUnauthorized) {
		t.Fatalf("expected title %q, got %q", http.StatusText(http.StatusUnauthorized), body.Title)
	}
	if body.Message != "O cabeçalho de autorização é obrigatório." {
		t.Fatalf("unexpected message: %q", body.Message)
	}
	if body.Type != "/errors/unauthorized" {
		t.Fatalf("expected type /errors/unauthorized, got %q", body.Type)
	}
}

// TestSendProblemWithDetails valida detalhes granulares tipados (validação).
func TestSendProblemWithDetails(t *testing.T) {
	rr := httptest.NewRecorder()
	p := errors.NewProblem(http.StatusBadRequest, "validation_failed", "Erro de validação.")
	p.Details = []errors.ProblemDetail{
		{Field: "email", Code: "email_invalid", Message: "Email inválido."},
	}
	SendProblem(rr, p)

	var body errors.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Details) != 1 || body.Details[0].Field != "email" {
		t.Fatalf("unexpected details: %#v", body.Details)
	}
}

// TestSendProblemWithAction valida o campo action (sugestão de fluxo UX).
func TestSendProblemWithAction(t *testing.T) {
	rr := httptest.NewRecorder()
	p := errors.NewProblem(http.StatusUnauthorized, "session_expired", "Sessão expirada.")
	p.Action = &errors.ProblemAction{Type: "relogin", URI: "app://auth/login"}
	SendProblem(rr, p)

	var body errors.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Action == nil || body.Action.Type != "relogin" {
		t.Fatalf("expected relogin action, got %#v", body.Action)
	}
}

// TestSendErrorMinimal garante que SendError produz um Problem mínimo e
// seguro, com Content-Type application/problem+json (retrocompatibilidade
// das chamadas simples).
func TestSendErrorMinimal(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, "erro simples", http.StatusInternalServerError)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", ct)
	}

	var body errors.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Message != "erro simples" {
		t.Fatalf("unexpected message: %q", body.Message)
	}
	if body.TraceID != "" {
		t.Fatalf("expected empty trace_id, got %q", body.TraceID)
	}
	if len(body.Details) != 0 {
		t.Fatalf("expected no details, got %#v", body.Details)
	}
}

// TestSelectFieldsSelectsOnlyRequested valida a projeção de campos (defesa de
// payload mobile) via reflexão: seleciona apenas os campos pedidos.
type selectFixture struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageUrl string `json:"imageUrl"`
	Secret   string `json:"secret"`
}

func TestSelectFieldsSelectsOnlyRequested(t *testing.T) {
	beers := []selectFixture{
		{ID: "1", Name: "Heineken", ImageUrl: "https://x/a.jpg", Secret: "no-leak"},
	}
	out := SelectFields(beers, "id,name")

	list, ok := out.([]any)
	if !ok {
		t.Fatalf("expected []any projection, got %T", out)
	}
	proj, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map projection, got %T", list[0])
	}
	if proj["id"] != "1" || proj["name"] != "Heineken" {
		t.Fatalf("unexpected projected values: %#v", proj)
	}
	if _, present := proj["secret"]; present {
		t.Fatal("secret should be excluded by projection")
	}
	if _, present := proj["imageUrl"]; present {
		t.Fatal("imageUrl should be excluded by projection")
	}
}

func TestSelectFieldsEmptyReturnsOriginal(t *testing.T) {
	beers := []selectFixture{{ID: "1", Name: "X"}}
	out := SelectFields(beers, "")
	got, ok := out.([]selectFixture)
	if !ok {
		t.Fatalf("expected original []selectFixture, got %T", out)
	}
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

func TestSelectFieldsUnknownFieldIgnored(t *testing.T) {
	beers := []selectFixture{{ID: "1", Name: "X"}}
	out := SelectFields(beers, "id,nonexistent")
	list := out.([]any)
	proj := list[0].(map[string]any)
	if _, present := proj["id"]; !present {
		t.Fatal("expected id to be present")
	}
	if _, present := proj["nonexistent"]; present {
		t.Fatal("nonexistent field must not appear")
	}
}

var _ = strings.Contains
