package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSendErrorWithErrorCode valida que WithErrorCode injeta o código estável
// (ex: "DUPLICATE_BEER") no envelope, sem vazar a causa raiz.
func TestSendErrorWithErrorCode(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, "cerveja duplicada", http.StatusConflict, WithErrorCode("DUPLICATE_BEER"))

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}

	var body errorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error != "cerveja duplicada" {
		t.Fatalf("unexpected error: %q", body.Error)
	}
	if body.Code != "DUPLICATE_BEER" {
		t.Fatalf("expected code DUPLICATE_BEER, got %q", body.Code)
	}
}

// TestSendErrorWithDetail valida que WithDetail injeta o payload seguro
// (ex: sugestões) sem expor erro interno.
func TestSendErrorWithDetail(t *testing.T) {
	rr := httptest.NewRecorder()
	detail := []map[string]any{{"id": "12", "name": "Heineken Long Neck"}}
	SendError(rr, "já existe", http.StatusConflict, WithDetail(detail))

	var body errorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Detail == nil {
		t.Fatal("expected detail to be set")
	}
	got, ok := body.Detail.([]any)
	if !ok || len(got) != 1 {
		t.Fatalf("unexpected detail: %#v", body.Detail)
	}
}

// TestSendErrorWithAllOptions valida a combinação de código + detail.
func TestSendErrorWithAllOptions(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, "msg", http.StatusBadRequest,
		WithErrorCode("BAD"),
		WithDetail(map[string]any{"field": "name"}))

	var body errorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != "BAD" {
		t.Fatalf("expected code BAD, got %q", body.Code)
	}
	if body.Detail == nil {
		t.Fatal("expected detail set")
	}
}

// TestSendErrorNoLeak garante que, SEM opções, o envelope não inclui code/detail
// (omitempty), evitando vazamento de campos vazios.
func TestSendErrorNoLeak(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, "erro simples", http.StatusInternalServerError)

	var body errorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != "" {
		t.Fatalf("expected empty code, got %q", body.Code)
	}
	if body.Detail != nil {
		t.Fatalf("expected nil detail, got %#v", body.Detail)
	}
}

// TestSelectFields valida a projeção de campos (defesa de payload mobile) via
// reflexão: seleciona apenas os campos pedidos e respeita o cap de 32 campos.
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
	// Sem fields, devolve o payload original (sem alocação de projeção).
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
