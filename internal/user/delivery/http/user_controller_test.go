package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"beer-review-app/internal/user/model"
	appErrors "beer-review-app/pkg/errors"

	"github.com/stretchr/testify/mock"
)

// MockUserUsecase é uma implementação mock da interface usecase.UserUsecase
// (mocking via interface, zero-dependency de DB).
type MockUserUsecase struct {
	mock.Mock
}

func (m *MockUserUsecase) Register(ctx context.Context, u model.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *MockUserUsecase) Login(ctx context.Context, email, password string) (string, error) {
	args := m.Called(ctx, email, password)
	return args.String(0), args.Error(1)
}

func (m *MockUserUsecase) OAuthLogin(ctx context.Context, provider, idToken string) (string, error) {
	args := m.Called(ctx, provider, idToken)
	return args.String(0), args.Error(1)
}

func (m *MockUserUsecase) GetProfile(ctx context.Context, id string) (model.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *MockUserUsecase) UpdateProfile(ctx context.Context, id string, u model.User) error {
	args := m.Called(ctx, id, u)
	return args.Error(0)
}

func (m *MockUserUsecase) DeleteAccount(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserUsecase) SeedAdmin(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockUserUsecase) SubscribePush(ctx context.Context, userID string, sub model.PushSubscription) error {
	args := m.Called(ctx, userID, sub)
	return args.Error(0)
}

func (m *MockUserUsecase) UnsubscribePush(ctx context.Context, userID, endpoint string) error {
	args := m.Called(ctx, userID, endpoint)
	return args.Error(0)
}

func (m *MockUserUsecase) ListPushSubscriptions(ctx context.Context, userID string) ([]model.PushSubscription, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.PushSubscription), args.Error(1)
}

func newUserController(mu *MockUserUsecase) *UserController {
	return NewUserController(mu, nil)
}

// --- Register ---

func TestUserRegisterSuccess(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("Register", mock.Anything, mock.Anything).Return(nil)

	body, _ := json.Marshal(model.User{Username: "bob", Email: "bob@example.com", Password: "secret1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestUserRegisterInvalidBody(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewBufferString("not json"))
	rr := httptest.NewRecorder()

	c.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUserRegisterValidationFails(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	// Email inválido → validator rejeita com 400 (antes do usecase).
	body, _ := json.Marshal(model.User{Username: "bob", Email: "not-an-email", Password: "secret1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	mu.AssertNotCalled(t, "Register", mock.Anything, mock.Anything)
}

func TestUserRegisterAppError(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("Register", mock.Anything, mock.Anything).
		Return(appErrors.NewAppError(400, "email already registered", nil))

	body, _ := json.Marshal(model.User{Username: "bob", Email: "bob@example.com", Password: "secret1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- Login ---

func TestUserLoginSuccess(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("Login", mock.Anything, "bob@example.com", "secret1").Return("jwt-token-123", nil)

	body, _ := json.Marshal(map[string]string{"email": "bob@example.com", "password": "secret1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Token != "jwt-token-123" {
		t.Fatalf("expected token in response, got %q", resp.Token)
	}
}

func TestUserLoginInvalidFormat(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	// Email em formato inválido → 400 (validação antes do usecase).
	body, _ := json.Marshal(map[string]string{"email": "bad", "password": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUserLoginUnauthorized(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("Login", mock.Anything, "bob@example.com", "wrong").
		Return("", appErrors.NewAppError(401, "invalid credentials", nil))

	body, _ := json.Marshal(map[string]string{"email": "bob@example.com", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.Login(rr, req)

	// O handler NÃO vaza a mensagem do AppError (segurança): responde 401
	// genérico "Invalid credentials".
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("Invalid credentials")) {
		t.Fatalf("expected generic message, got %s", rr.Body.String())
	}
}

// --- GetProfile ---

func TestUserGetProfileSuccess(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	want := model.User{ID: "u1", Username: "bob", Email: "bob@example.com"}
	mu.On("GetProfile", mock.Anything, "u1").Return(want, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1", nil)
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.GetProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got model.User
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.ID != "u1" {
		t.Fatalf("expected user u1, got %q", got.ID)
	}
}

func TestUserGetProfileMissingID(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", nil)
	rr := httptest.NewRecorder()

	c.GetProfile(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUserGetProfileNotFound(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("GetProfile", mock.Anything, "missing").
		Return(model.User{}, appErrors.NewAppError(404, "user not found", nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/missing", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()

	c.GetProfile(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// --- UpdateProfile ---

func TestUserUpdateProfileSuccess(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("UpdateProfile", mock.Anything, "u1", mock.Anything).Return(nil)

	body, _ := json.Marshal(model.User{Username: "bob2", Email: "bob2@example.com", Password: "secret1"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1", bytes.NewBuffer(body))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.UpdateProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestUserUpdateProfileMissingID(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/", bytes.NewBufferString("{}"))
	rr := httptest.NewRecorder()

	c.UpdateProfile(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUserUpdateProfileInvalidBody(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1", bytes.NewBufferString("garbage"))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.UpdateProfile(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// Invariante de Poluição / Escalonamento de Privilégio: o cliente não pode
// promover-se a admin nem forjar o id no PUT /users/{id}.
func TestUserUpdateProfile_RejectsRoleForge(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	var captured model.User
	mu.On("UpdateProfile", mock.Anything, "u1", mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(2).(model.User)
		}).
		Return(nil)

	body, _ := json.Marshal(map[string]string{"username": "vitinho", "email": "v@x.com", "role": "admin", "id": "forged"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1", bytes.NewBuffer(body))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.UpdateProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if captured.Role == "admin" {
		t.Errorf("poluição: cliente conseguiu forjar role=admin (%q)", captured.Role)
	}
	if captured.ID == "forged" {
		t.Errorf("poluição: cliente conseguiu forjar id (%q)", captured.ID)
	}
}

// --- DeleteAccount ---

func TestUserDeleteAccountSuccess(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("DeleteAccount", mock.Anything, "u1").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1", nil)
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.DeleteAccount(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestUserDeleteAccountMissingID(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/", nil)
	rr := httptest.NewRecorder()

	c.DeleteAccount(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUserDeleteAccountFailure(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("DeleteAccount", mock.Anything, "u1").
		Return(appErrors.NewAppError(500, "failed to delete account", nil))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1", nil)
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.DeleteAccount(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// --- OAuth ---

func TestUserOAuthSuccess(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("OAuthLogin", mock.Anything, "google", "idtok-123").Return("jwt-session", nil)

	body, _ := json.Marshal(map[string]string{"provider": "google", "id_token": "idtok-123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/oauth", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.OAuth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Token != "jwt-session" {
		t.Fatalf("expected session token, got %q", resp.Token)
	}
}

func TestUserOAuthUnauthorized(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("OAuthLogin", mock.Anything, "google", "bad").
		Return("", appErrors.NewAppError(401, "invalid id_token", nil))

	body, _ := json.Marshal(map[string]string{"provider": "google", "id_token": "bad"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/oauth", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.OAuth(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestUserOAuthBadRequest(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	body, _ := json.Marshal(map[string]string{"provider": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/oauth", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.OAuth(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- Push Subscriptions ---

func TestUserSubscribePushSuccess(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("SubscribePush", mock.Anything, "u1", model.PushSubscription{
		Endpoint: "https://push.example.com",
		P256DH:   "p256dh",
		Auth:     "auth",
	}).Return(nil)

	body, _ := json.Marshal(model.PushSubscription{
		Endpoint: "https://push.example.com",
		P256DH:   "p256dh",
		Auth:     "auth",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/push/subscribe", bytes.NewBuffer(body))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.SubscribePush(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestUserSubscribePushMissingID(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	body, _ := json.Marshal(model.PushSubscription{
		Endpoint: "https://push.example.com",
		P256DH:   "p256dh",
		Auth:     "auth",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users//push/subscribe", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.SubscribePush(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUserSubscribePushInvalidBody(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/push/subscribe", bytes.NewBufferString("not json"))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.SubscribePush(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUserUnsubscribePushSuccess(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	mu.On("UnsubscribePush", mock.Anything, "u1", "https://push.example.com").Return(nil)

	body, _ := json.Marshal(map[string]string{"endpoint": "https://push.example.com"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/push/unsubscribe", bytes.NewBuffer(body))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.UnsubscribePush(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestUserUnsubscribePushMissingID(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	body, _ := json.Marshal(map[string]string{"endpoint": "https://push.example.com"})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users//push/unsubscribe", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	c.UnsubscribePush(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUserListPushSubscriptionsSuccess(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	subs := []model.PushSubscription{
		{Endpoint: "https://push.example.com", P256DH: "p256dh", Auth: "auth"},
	}
	mu.On("ListPushSubscriptions", mock.Anything, "u1").Return(subs, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/push/subscriptions", nil)
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	c.ListPushSubscriptions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Subscriptions []model.PushSubscription `json:"subscriptions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(resp.Subscriptions))
	}
}

func TestUserListPushSubscriptionsMissingID(t *testing.T) {
	mu := new(MockUserUsecase)
	c := newUserController(mu)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users//push/subscriptions", nil)
	rr := httptest.NewRecorder()

	c.ListPushSubscriptions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
