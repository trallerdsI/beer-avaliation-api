package response

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestETagForVersion(t *testing.T) {
	etag := ETagForVersion("abc:2026-07-18T17:00:00Z")
	if etag != `"abc:2026-07-18T17:00:00Z"` {
		t.Fatalf("ETagForVersion = %q", etag)
	}
}

func TestETagForContent_Deterministic(t *testing.T) {
	a := ETagForContent([]byte(`{"page":1}`))
	b := ETagForContent([]byte(`{"page":1}`))
	if a != b {
		t.Fatalf("ETag deveria ser determinístico: %q != %q", a, b)
	}
	c := ETagForContent([]byte(`{"page":2}`))
	if a == c {
		t.Fatalf("ETag deveria mudar com o conteúdo")
	}
	if len(a) < 3 || a[0] != '"' || a[len(a)-1] != '"' {
		t.Fatalf("ETag forte deve estar entre aspas: %q", a)
	}
}

func TestIfNoneMatchMatches(t *testing.T) {
	etag := `"v1"`
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	if IfNoneMatchMatches(r, etag) {
		t.Fatal("sem header não deve bater")
	}

	r = httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("If-None-Match", `"v1"`)
	if !IfNoneMatchMatches(r, etag) {
		t.Fatal("match exato deve bater")
	}

	r = httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("If-None-Match", `"other", "v1"`)
	if !IfNoneMatchMatches(r, etag) {
		t.Fatal("lista com match deve bater")
	}

	r = httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("If-None-Match", "*")
	if !IfNoneMatchMatches(r, etag) {
		t.Fatal("'*' deve bater")
	}

	r = httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("If-None-Match", `"v2"`)
	if IfNoneMatchMatches(r, etag) {
		t.Fatal("mismatch não deve bater")
	}
}

func TestSendNotModified_StatusAndNoBody(t *testing.T) {
	w := httptest.NewRecorder()
	SendNotModified(w, `"v1"`)
	if w.Code != http.StatusNotModified {
		t.Fatalf("status = %d, queria 304", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("304 não deve ter body, len = %d", w.Body.Len())
	}
	if w.Header().Get("ETag") != `"v1"` {
		t.Fatalf("ETag ausente no 304: %q", w.Header().Get("ETag"))
	}
}

func TestSetCacheHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	SetCacheHeaders(w, `"v1"`, 300, true)
	if w.Header().Get("ETag") != `"v1"` {
		t.Fatal("ETag não setado")
	}
	if w.Header().Get("Cache-Control") != "public, max-age=300, must-revalidate" {
		t.Fatalf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
}
