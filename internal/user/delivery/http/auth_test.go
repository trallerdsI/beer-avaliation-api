package http

import (
	"net/http"
	"beer-review-app/internal/user/model"
	"net/http/httptest"
	"testing"

	"beer-review-app/internal/user/usecase"
	"beer-review-app/pkg/auth"
	"beer-review-app/pkg/middleware"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestAuthMiddleware_MissingHeader valida que o middleware bloqueia requisições
// sem Authorization.
func TestAuthMiddleware_MissingHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next não deveria ser chamado sem token")
	})
	h := middleware.Auth(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestAuthMiddleware_InvalidFormat valida o formato "Bearer <token>".
func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next não deveria ser chamado com formato inválido")
	})
	h := middleware.Auth(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	req.Header.Set("Authorization", "Token xyz")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestAuthMiddleware_InvalidToken valida rejeição de token malformado.
func TestAuthMiddleware_InvalidToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next não deveria ser chamado com token inválido")
	})
	h := middleware.Auth(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid.jwt.token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestAuthMiddleware_ValidTokenHappyPath valida o fluxo completo: token
// válido (gerado com stdlib HS256) -> rota protegida -> usecase chamado.
func TestAuthMiddleware_ValidTokenHappyPath(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key")
	auth.JWTSecretForTest() // recarrega o segredo para o teste

	token, err := auth.GenerateToken("user-1")
	assert.NoError(t, err)

	mockUserUsecase = new(usecase.MockUserUsecase)
	mockUserUsecase.On("GetProfile", mock.Anything, "user-1").
		Return(model.User{ID: "user-1", Username: "john"}, nil)
	c := newUserController()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1", nil)
	req.SetPathValue("id", "user-1")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	// Catena o middleware Auth à rota protegida.
	middleware.Auth(c.GetProfile).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockUserUsecase.AssertExpectations(t)
}
