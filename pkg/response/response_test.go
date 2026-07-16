package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSendErrorSemantics valida o envelope de erro e garante que NÃO alocamos
// um map[string]string por request (a struct errorEnvelope vive na stack).
func TestSendErrorSemantics(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, "invalid credentials", http.StatusUnauthorized)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Error != "invalid credentials" {
		t.Fatalf("expected error message, got %q", body.Error)
	}
	// O envelope deve ser um objeto único com a chave "error".
	if !strings.Contains(rr.Body.String(), `"error":"invalid credentials"`) {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestSendResponsePassthrough(t *testing.T) {
	rr := httptest.NewRecorder()
	SendResponse(rr, http.StatusOK, map[string]int{"total": 3})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"total":3`) {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}
