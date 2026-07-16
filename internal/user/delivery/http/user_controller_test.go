package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/usecase"
	"beer-review-app/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var mockUserUsecase = new(usecase.MockUserUsecase)

func newUserController() *UserController {
	return NewUserController(mockUserUsecase, slog.Default())
}

// ---------------------------------------------------------------------------
// POST /api/v1/users/register
// ---------------------------------------------------------------------------

func TestRegister_Success(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	mockUserUsecase.On("Register", mock.Anything, mock.Anything).Return(nil)

	body, _ := json.Marshal(model.User{Username: "johndoe", Email: "a@b.com", Password: "secret123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	c.Register(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Contains(t, rr.Body.String(), "registered successfully")
}

func TestRegister_InvalidBody(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader([]byte("not-json{")))
	rr := httptest.NewRecorder()
	c.Register(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegister_ValidationFails(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	// Username curto + email inválido + password curta => 400.
	body, _ := json.Marshal(model.User{Username: "ab", Email: "x", Password: "1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	c.Register(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	mockUserUsecase.AssertNotCalled(t, "Register", mock.Anything, mock.Anything)
}

func TestRegister_UsecaseError(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	mockUserUsecase.On("Register", mock.Anything, mock.Anything).
		Return(errors.NewAppError(400, "email already registered", nil))

	body, _ := json.Marshal(model.User{Username: "johndoe", Email: "a@b.com", Password: "secret123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	c.Register(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---------------------------------------------------------------------------
// POST /api/v1/users/login
// ---------------------------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	mockUserUsecase.On("Login", mock.Anything, "a@b.com", "secret123").
		Return("jwt-token-xyz", nil)

	body, _ := json.Marshal(map[string]string{"email": "a@b.com", "password": "secret123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	c.Login(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "jwt-token-xyz")
}

func TestLogin_ValidationFails(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	// Email vazio => falha de validação antes do usecase.
	body, _ := json.Marshal(map[string]string{"email": "", "password": "secret123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	c.Login(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	mockUserUsecase.On("Login", mock.Anything, mock.Anything, mock.Anything).
		Return("", errors.NewAppError(401, "invalid credentials", nil))

	body, _ := json.Marshal(map[string]string{"email": "a@b.com", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	c.Login(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// ---------------------------------------------------------------------------
// GET/PUT/DELETE /api/v1/users/{id}
// ---------------------------------------------------------------------------

func TestGetProfile_Success(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	mockUserUsecase.On("GetProfile", mock.Anything, "1").
		Return(model.User{ID: "1", Username: "johndoe"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	c.GetProfile(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "johndoe")
}

func TestGetProfile_NotFound(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	mockUserUsecase.On("GetProfile", mock.Anything, "1").
		Return(model.User{}, errors.NewAppError(404, "user not found", nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	c.GetProfile(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetProfile_MissingID(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", nil)
	rr := httptest.NewRecorder()
	c.GetProfile(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	mockUserUsecase.AssertNotCalled(t, "GetProfile", mock.Anything, mock.Anything)
}

func TestUpdateProfile_Success(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	mockUserUsecase.On("UpdateProfile", mock.Anything, "1", mock.Anything).Return(nil)

	body, _ := json.Marshal(model.User{Username: "newname", Email: "a@b.com"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	c.UpdateProfile(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateProfile_InvalidBody(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/1", bytes.NewReader([]byte("bad")))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	c.UpdateProfile(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDeleteAccount_Success(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	mockUserUsecase.On("DeleteAccount", mock.Anything, "1").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	c.DeleteAccount(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteAccount_UsecaseError(t *testing.T) {
	mockUserUsecase = new(usecase.MockUserUsecase)
	c := newUserController()

	mockUserUsecase.On("DeleteAccount", mock.Anything, "1").
		Return(errors.NewAppError(500, "delete failed", nil))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	c.DeleteAccount(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

