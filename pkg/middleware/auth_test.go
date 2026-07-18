package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"beer-review-app/internal/user/model"
	"beer-review-app/pkg/auth"
)

// newValidToken gera um JWT válido para os testes, recarregando o segredo HS256.
func newValidToken(t *testing.T, userID, role string) string {
	t.Setenv("JWT_SECRET", "test-secret-key")
	auth.JWTSecretForTest()
	token, err := auth.GenerateToken(userID, role)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func TestAuthNoHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)

	Auth(okHandler)(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMalformedHeader(t *testing.T) {
	cases := []struct {
		name   string
		header string
	}{
		{"empty scheme", "token-without-scheme"},
		{"wrong scheme", "Basic abcdef"},
		{"bearer without token", "Bearer "},
		{"empty header", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}

			Auth(okHandler)(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rr.Code)
			}
		})
	}
}

func TestAuthInvalidToken(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.token")

	Auth(okHandler)(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthValidTokenInjectsContext(t *testing.T) {
	token := newValidToken(t, "user-42", model.RoleUser)

	var gotUserID, gotRole string
	handler := func(w http.ResponseWriter, r *http.Request) {
		gotUserID, _ = UserIDFromContext(r.Context())
		gotRole, _ = RoleFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	Auth(handler)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotUserID != "user-42" {
		t.Fatalf("expected user_id user-42, got %q", gotUserID)
	}
	if gotRole != model.RoleUser {
		t.Fatalf("expected role %q, got %q", model.RoleUser, gotRole)
	}
}

func TestRequireAdminUnauthenticated(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)

	RequireAdmin(okHandler)(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAdminNonAdmin(t *testing.T) {
	token := newValidToken(t, "user-42", model.RoleUser)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	RequireAdmin(okHandler)(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestRequireAdminAdmin(t *testing.T) {
	token := newValidToken(t, "admin-1", model.RoleAdmin)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	RequireAdmin(okHandler)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestWithUserIDAndIsAdmin(t *testing.T) {
	// Sem role → não é admin.
	ctx := WithUserID(context.Background(), "u1", "")
	if id, ok := UserIDFromContext(ctx); !ok || id != "u1" {
		t.Fatalf("expected user_id u1, got %q (ok=%v)", id, ok)
	}
	if IsAdmin(ctx) {
		t.Fatal("expected non-admin for empty role")
	}

	// Com role admin → IsAdmin true.
	ctxAdmin := WithUserID(context.Background(), "u2", model.RoleAdmin)
	if !IsAdmin(ctxAdmin) {
		t.Fatal("expected admin for role admin")
	}
	if role, ok := RoleFromContext(ctxAdmin); !ok || role != model.RoleAdmin {
		t.Fatalf("expected role admin, got %q (ok=%v)", role, ok)
	}
}

func TestContextHelpersMissing(t *testing.T) {
	ctx := context.Background()
	if id, ok := UserIDFromContext(ctx); ok || id != "" {
		t.Fatal("expected no user_id from empty context")
	}
	if role, ok := RoleFromContext(ctx); ok || role != "" {
		t.Fatal("expected no role from empty context")
	}
}
