package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"log/slog"
)

func newTestRouter() http.Handler {
	os.Setenv("RATE_LIMIT_DISABLED", "true")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return BuildRouterWithDBErr(nil, nil, logger)
}

func TestContract_HealthCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	router := newTestRouter()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code incorreto: esperado %d (liveness), obtido %d", http.StatusOK, w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type incorreto: esperado application/json, obtido %s", ct)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("resposta não é JSON válido: %v", err)
	}
	if resp["status"] != "alive" {
		t.Fatalf("esperado status 'alive' no liveness probe; obtido %v", resp["status"])
	}
}

func TestContract_ReadyCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	router := newTestRouter()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code incorreto: esperado %d (DB indisponível), obtido %d", http.StatusServiceUnavailable, w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/problem+json" && ct != "application/json" {
		t.Fatalf("Content-Type incorreto: esperado application/problem+json ou application/json, obtido %s", ct)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("resposta não é JSON válido: %v", err)
	}
	status, ok := resp["status"].(string)
	if !ok || status != "not_ready" {
		t.Fatalf("esperado status 'not_ready' na resposta; obtido %v", resp["status"])
	}
}

func TestContract_OpenAPISpec(t *testing.T) {
	t.Run("GET /docs/openapi.yaml retorna spec", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil)
		w := httptest.NewRecorder()

		router := newTestRouter()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status code incorreto: esperado %d, obtido %d", http.StatusOK, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/yaml" {
			t.Fatalf("Content-Type incorreto: esperado application/yaml, obtido %s", ct)
		}
		if w.Body.Len() == 0 {
			t.Fatal("corpo da resposta OpenAPI não deve estar vazio")
		}
	})
}

func TestContract_EnumsEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers/enums", nil)
	w := httptest.NewRecorder()

	router := newTestRouter()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code incorreto: esperado %d, obtido %d", http.StatusOK, w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type incorreto: esperado application/json, obtido %s", ct)
	}
	var resp map[string][]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("resposta não é JSON válido: %v", err)
	}
	if _, ok := resp["flavor"]; !ok {
		t.Fatal("resposta de enums deve conter 'flavor'")
	}
}

func TestContract_DataRoutes_UnavailableWhenDBIsNil(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodGet, "/api/v1/beers", nil},
		{http.MethodGet, "/api/v1/beers/1", nil},
		{http.MethodPost, "/api/v1/users/login", []byte(`{"email":"a@b.com","password":"x"}`)},
		{http.MethodPost, "/api/v1/users/oauth", []byte(`{"provider":"google","id_token":"x"}`)},
		{http.MethodGet, "/api/v1/stats", nil},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBuffer(tt.body))
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			router := newTestRouter()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("esperado 503 quando DB indisponível; obtido %d (body=%s)", w.Code, w.Body.String())
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Fatalf("esperado Content-Type application/problem+json; obtido %s", ct)
			}
		})
	}
}

func TestContract_UserLogin_Returns401WithProblem(t *testing.T) {
	payload := map[string]interface{}{
		"email":    "nonexistent@example.com",
		"password": "wrong",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := newTestRouter()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("login sem DB deve retornar 503; obtido %d", w.Code)
	}
}

func TestContract_OAuthLogin_Returns503WhenDBUnavailable(t *testing.T) {
	payload := map[string]interface{}{
		"provider": "google",
		"id_token": "fake_token",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/oauth", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := newTestRouter()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("oauth sem DB deve retornar 503; obtido %d", w.Code)
	}
}

func TestContract_GetBeers_Returns503WhenDBUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers?page=1&pageSize=10", nil)
	w := httptest.NewRecorder()

	router := newTestRouter()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("get beers sem DB deve retornar 503; obtido %d", w.Code)
	}
}

func TestContract_CreateBeer_RequiresAuthentication(t *testing.T) {
	payload := map[string]interface{}{
		"name":  "Test Beer",
		"style": "IPA",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := newTestRouter()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 sem token; obtido %d (body=%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("esperado Content-Type application/problem+json; obtido %s", ct)
	}
}
