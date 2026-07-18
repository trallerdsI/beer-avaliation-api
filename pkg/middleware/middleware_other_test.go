package middleware

import (
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCompressionMiddlewarePassthrough garante que, sem "gzip" no
// Accept-Encoding, a resposta passa intacta (sem Content-Encoding).
func TestCompressionMiddlewarePassthrough(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Encoding") != "" {
		t.Fatalf("expected no Content-Encoding, got %q", rr.Header().Get("Content-Encoding"))
	}
	if rr.Body.String() != "hello world" {
		t.Fatalf("expected body passthrough, got %q", rr.Body.String())
	}
}

// TestCompressionMiddlewareGzip valida compressão gzip: o cliente recebe o
// payload comprimido e o status correto (não 200 prematuro).
func TestCompressionMiddlewareGzip(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(strings.Repeat("beer", 100)))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip Content-Encoding, got %q", rr.Header().Get("Content-Encoding"))
	}
	if rr.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("expected Vary: Accept-Encoding, got %q", rr.Header().Get("Vary"))
	}

	gr, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	out, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	if string(out) != strings.Repeat("beer", 100) {
		t.Fatal("decompressed body mismatch")
	}
}

// TestCompressionMiddlewarePreservesErrorStatus garante que um handler que
// responde 404 não é silenciado como 200 pelo gzip (caso de Defesa-em-Profundidade).
func TestCompressionMiddlewarePreservesErrorStatus(t *testing.T) {
	handler := CompressionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// TestMetricsMiddleware verifica que a cadeia de métricas não quebra a resposta
// e captura o status do handler.
func TestMetricsMiddleware(t *testing.T) {
	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("x"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", rr.Code)
	}
	if rr.Body.String() != "x" {
		t.Fatalf("expected body x, got %q", rr.Body.String())
	}
}

// TestRequestIDMiddleware valida a injeção do X-Request-ID e do logger no
// contexto.
func TestRequestIDMiddleware(t *testing.T) {
	var gotLogger *slog.Logger
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLogger = LoggerFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
	if gotLogger == nil {
		t.Fatal("expected logger from context")
	}
}

// TestLoggerFromContextDefault garante fallback para slog.Default quando o
// contexto não tem logger.
func TestLoggerFromContextDefault(t *testing.T) {
	if LoggerFromContext(context.Background()) == nil {
		t.Fatal("expected non-nil default logger")
	}
}
