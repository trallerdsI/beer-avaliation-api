package errors

import (
	"os"
	"testing"
)

func TestProblemType(t *testing.T) {
	if got := ProblemType("BEER_NOT_FOUND"); got != "/errors/BEER_NOT_FOUND" {
		t.Fatalf("expected /errors/BEER_NOT_FOUND, got %q", got)
	}
}

func TestSetProblemTypeBase(t *testing.T) {
	SetProblemTypeBase("/custom")
	if got := ProblemType("X"); got != "/custom/X" {
		t.Fatalf("expected /custom/X, got %q", got)
	}
	SetProblemTypeBase("/errors")
}

func TestHTTPStatusSlug(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{400, "bad_request"},
		{401, "unauthorized"},
		{403, "forbidden"},
		{404, "not_found"},
		{409, "conflict"},
		{422, "unprocessable_entity"},
		{429, "too_many_requests"},
		{500, "internal_server_error"},
		{503, "service_unavailable"},
		{999, "error"},
	}
	for _, tc := range cases {
		if got := HTTPStatusSlug(tc.status); got != tc.want {
			t.Fatalf("HTTPStatusSlug(%d): got %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestStatusText(t *testing.T) {
	if got := statusText(404); got != "Not Found" {
		t.Fatalf("expected 'Not Found', got %q", got)
	}
}

func TestNewProblem(t *testing.T) {
	p := NewProblem(404, "BEER_NOT_FOUND", "beer not found")
	if p.Status != 404 {
		t.Fatalf("expected status 404, got %d", p.Status)
	}
	if p.Code != "BEER_NOT_FOUND" {
		t.Fatalf("expected code BEER_NOT_FOUND, got %q", p.Code)
	}
	if p.Detail != "beer not found" {
		t.Fatalf("expected detail 'beer not found', got %q", p.Detail)
	}
	if p.Title == "" {
		t.Fatal("expected non-empty title")
	}
	if p.Type == "" {
		t.Fatal("expected non-empty type")
	}
}

func TestGetProblemTypeBaseFromEnv(t *testing.T) {
	orig := os.Getenv("PROBLEM_TYPE_BASE")
	os.Setenv("PROBLEM_TYPE_BASE", "/api/errors")
	defer os.Setenv("PROBLEM_TYPE_BASE", orig)
	// Recompute via package-level init by setting env before calling.
	// Since problemTypeBase is initialized at startup, we test via SetProblemTypeBase
	// and ProblemType.
	SetProblemTypeBase("/api/errors")
	if got := ProblemType("X"); got != "/api/errors/X" {
		t.Fatalf("expected /api/errors/X, got %q", got)
	}
	SetProblemTypeBase("/errors")
}
