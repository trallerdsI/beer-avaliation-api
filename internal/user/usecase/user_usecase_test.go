package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/repository"
	"beer-review-app/pkg/auth"
	appErrors "beer-review-app/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock repositório (interface-based, zero-dependency de DB) ---

type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) Create(ctx context.Context, u model.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (model.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (model.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *mockUserRepo) GetByExternal(ctx context.Context, provider, externalSub string) (model.User, error) {
	args := m.Called(ctx, provider, externalSub)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *mockUserRepo) UpsertByExternal(ctx context.Context, u model.User) (model.User, error) {
	args := m.Called(ctx, u)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *mockUserRepo) Update(ctx context.Context, id string, u model.User) error {
	args := m.Called(ctx, id, u)
	return args.Error(0)
}

func (m *mockUserRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepo) List(ctx context.Context, page, pageSize int) ([]model.User, int, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]model.User), args.Int(1), args.Error(2)
}

func (m *mockUserRepo) CreatePushSubscription(ctx context.Context, sub model.PushSubscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *mockUserRepo) ListPushSubscriptions(ctx context.Context, userID string) ([]model.PushSubscription, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.PushSubscription), args.Error(1)
}

func (m *mockUserRepo) DeletePushSubscription(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepo) DeletePushSubscriptionByEndpoint(ctx context.Context, userID, endpoint string) error {
	args := m.Called(ctx, userID, endpoint)
	return args.Error(0)
}

func (m *mockUserRepo) GetMemberSince(ctx context.Context, userID string) (time.Time, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *mockUserRepo) ExecInTx(ctx context.Context, fn func(ctx context.Context, txRepo repository.UserRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func newUserUsecase(repo *mockUserRepo) UserUsecase {
	return NewUserUsecase(repo)
}

// --- Register ---

func TestRegisterDuplicateEmail(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	existing := model.User{ID: "1", Email: "dup@example.com", Username: "dup"}
	repo.On("GetByEmail", mock.Anything, "dup@example.com").Return(existing, nil)

	err := uc.Register(context.Background(), model.User{Email: "dup@example.com", Password: "secret1"})

	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 400, appErr.Code)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestRegisterPasswordTooLong(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("GetByEmail", mock.Anything, "long@example.com").Return(model.User{}, errors.New("user not found"))

	err := uc.Register(context.Background(), model.User{
		Email:    "long@example.com",
		Password: string(make([]byte, 73)),
	})

	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 400, appErr.Code)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestRegisterSuccess(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("GetByEmail", mock.Anything, "new@example.com").Return(model.User{}, errors.New("user not found"))
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	err := uc.Register(context.Background(), model.User{
		Username: "newbie",
		Email:    "new@example.com",
		Password: "secret1",
	})

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestRegisterCreateFails(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("GetByEmail", mock.Anything, "fail@example.com").Return(model.User{}, errors.New("user not found"))
	repo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db error"))

	err := uc.Register(context.Background(), model.User{Email: "fail@example.com", Password: "secret1"})

	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 500, appErr.Code)
}

// --- Login ---

func TestLoginPasswordTooLong(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	_, err := uc.Login(context.Background(), "x@example.com", string(make([]byte, 73)))

	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 401, appErr.Code)
}

func TestLoginUserNotFound(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("GetByEmail", mock.Anything, "nope@example.com").Return(model.User{}, errors.New("user not found"))

	_, err := uc.Login(context.Background(), "nope@example.com", "secret1")

	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 401, appErr.Code)
}

func TestLoginWrongPassword(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	hashed := "$2a$10$muEIEoMqFBHIBsb1naTmBuzHrijo7LpbQn/eyTlrMBQLt6MbvuLjO"
	repo.On("GetByEmail", mock.Anything, "user@example.com").Return(model.User{
		ID:       "u1",
		Email:    "user@example.com",
		Password: hashed,
		Role:     model.RoleUser,
	}, nil)

	_, err := uc.Login(context.Background(), "user@example.com", "wrongpass")

	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 401, appErr.Code)
}

func TestLoginSuccess(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	// Hash gerado via bcrypt para "secret1".
	hashed := "$2a$10$muEIEoMqFBHIBsb1naTmBuzHrijo7LpbQn/eyTlrMBQLt6MbvuLjO"
	repo.On("GetByEmail", mock.Anything, "user@example.com").Return(model.User{
		ID:       "u1",
		Email:    "user@example.com",
		Password: hashed,
		Role:     model.RoleUser,
	}, nil)

	t.Setenv("JWT_SECRET", "test-secret-key")
	// Regenera o segredo (loadJWTSecret corre no init) para o teste assinar o token.
	auth.JWTSecretForTest()

	token, err := uc.Login(context.Background(), "user@example.com", "secret1")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

// --- GetProfile / UpdateProfile / DeleteAccount ---

func TestGetProfileNotFound(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("GetByID", mock.Anything, "missing").Return(model.User{}, errors.New("user not found"))

	_, err := uc.GetProfile(context.Background(), "missing")

	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 404, appErr.Code)
}

func TestGetProfileSuccess(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	want := model.User{ID: "u1", Username: "bob", Email: "bob@example.com"}
	repo.On("GetByID", mock.Anything, "u1").Return(want, nil)

	got, err := uc.GetProfile(context.Background(), "u1")

	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestUpdateProfileSuccess(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("Update", mock.Anything, "u1", mock.Anything).Return(nil)

	err := uc.UpdateProfile(context.Background(), "u1", model.User{Username: "bob2"})
	assert.NoError(t, err)
}

func TestUpdateProfileFailure(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("Update", mock.Anything, "u1", mock.Anything).Return(errors.New("db error"))

	err := uc.UpdateProfile(context.Background(), "u1", model.User{Username: "bob2"})
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 500, appErr.Code)
}

func TestDeleteAccountSuccess(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("Delete", mock.Anything, "u1").Return(nil)

	err := uc.DeleteAccount(context.Background(), "u1")
	assert.NoError(t, err)
}

func TestDeleteAccountFailure(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("Delete", mock.Anything, "u1").Return(errors.New("db error"))

	err := uc.DeleteAccount(context.Background(), "u1")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 500, appErr.Code)
}

// --- SeedAdmin ---

func TestSeedAdminSkippedWhenEnvUnset(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	t.Setenv("ADMIN_EMAIL", "")
	t.Setenv("ADMIN_PASSWORD", "")

	err := uc.SeedAdmin(context.Background())
	assert.NoError(t, err)
	repo.AssertNotCalled(t, "GetByEmail", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSeedAdminPromotesExistingUser(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "supersecret")

	repo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	err := uc.SeedAdmin(context.Background())
	assert.NoError(t, err)
}

func TestSeedAdminCreatesNewUser(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "supersecret")

	repo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	err := uc.SeedAdmin(context.Background())
	assert.NoError(t, err)
}

func TestSeedAdminAlreadyAdminNoop(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "supersecret")

	repo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	err := uc.SeedAdmin(context.Background())
	assert.NoError(t, err)
}

// --- OAuthLogin (RFC 6749 / OIDC) ---

func TestOAuthLoginSuccess(t *testing.T) {
	// Injeta um verifier fake no registry global de auth e sobrescreve o
	// Verify via hook (sem rede).
	auth.OIDC().RegisterVerifier("google", "https://accounts.google.com", "aud-test", "")
	v := auth.OIDC().Verifier("google")
	v.SetVerifyOverride(func(ctx context.Context, idToken string) (auth.OIDCClaims, error) {
		return auth.OIDCClaims{Sub: "google|sub1", Email: "oauth@example.com", Username: "OAuth User"}, nil
	})

	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("UpsertByExternal", mock.Anything, mock.Anything).Return(model.User{
		ID: "u-oauth", Role: model.RoleUser,
	}, nil)

	t.Setenv("JWT_SECRET", "test-secret-key")
	auth.JWTSecretForTest()

	token, err := uc.OAuthLogin(context.Background(), "google", "fake-id-token")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	repo.AssertCalled(t, "UpsertByExternal", mock.Anything, mock.Anything)
}

func TestOAuthLoginUnsupportedProvider(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	_, err := uc.OAuthLogin(context.Background(), "unknown", "tok")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 400, appErr.Code)
	repo.AssertNotCalled(t, "UpsertByExternal", mock.Anything, mock.Anything)
}

func TestOAuthLoginInvalidToken(t *testing.T) {
	auth.OIDC().RegisterVerifier("google", "https://accounts.google.com", "aud-test", "")
	v := auth.OIDC().Verifier("google")
	v.SetVerifyOverride(func(ctx context.Context, idToken string) (auth.OIDCClaims, error) {
		return auth.OIDCClaims{}, errors.New("invalid id_token")
	})

	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	_, err := uc.OAuthLogin(context.Background(), "google", "bad")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 401, appErr.Code)
	repo.AssertNotCalled(t, "UpsertByExternal", mock.Anything, mock.Anything)
}
