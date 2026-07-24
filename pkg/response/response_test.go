package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSendErrorSemantics valida o envelope RFC 7807 produzido por SendError:
// Content-Type application/problem+json e mensagem no campo "detail".
func TestSendErrorSemantics(t *testing.T) {
	rr := httptest.NewRecorder()
	SendError(rr, "invalid credentials", http.StatusUnauthorized)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %q", ct)
	}

	var body struct {
		Detail string `json:"detail"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Detail != "invalid credentials" {
		t.Fatalf("expected detail, got %q", body.Detail)
	}
	if !strings.Contains(rr.Body.String(), `"detail":"invalid credentials"`) {
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
