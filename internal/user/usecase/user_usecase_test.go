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
	"beer-review-app/pkg/middleware"

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

func (m *mockUserRepo) CreateRefreshToken(ctx context.Context, token model.RefreshToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *mockUserRepo) GetRefreshTokenByHash(ctx context.Context, userID, tokenHash string) (model.RefreshToken, error) {
	args := m.Called(ctx, userID, tokenHash)
	return args.Get(0).(model.RefreshToken), args.Error(1)
}

func (m *mockUserRepo) RevokeRefreshToken(ctx context.Context, userID, tokenHash string) error {
	args := m.Called(ctx, userID, tokenHash)
	return args.Error(0)
}

func (m *mockUserRepo) RevokeAllRefreshTokens(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
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

	repo.On("GetByEmail", mock.Anything, "long@example.com").Return(model.User{}, repository.ErrUserNotFound)

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

	repo.On("GetByEmail", mock.Anything, "new@example.com").Return(model.User{}, repository.ErrUserNotFound)
	var created model.User
	repo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		created = args.Get(1).(model.User)
	}).Return(nil)

	err := uc.Register(context.Background(), model.User{
		Username: "newbie",
		Email:    "new@example.com",
		Password: "secret1",
		Role:     model.RoleAdmin,
	})

	assert.NoError(t, err)
	assert.Equal(t, model.RoleUser, created.Role)
	repo.AssertExpectations(t)
}

func TestRegisterCreateFails(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("GetByEmail", mock.Anything, "fail@example.com").Return(model.User{}, repository.ErrUserNotFound)
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

	repo.On("GetByEmail", mock.Anything, "nope@example.com").Return(model.User{}, repository.ErrUserNotFound)

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

	hashed := "$2a$10$muEIEoMqFBHIBsb1naTmBuzHrijo7LpbQn/eyTlrMBQLt6MbvuLjO"
	repo.On("GetByEmail", mock.Anything, "user@example.com").Return(model.User{
		ID:       "u1",
		Email:    "user@example.com",
		Password: hashed,
		Role:     model.RoleUser,
	}, nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)

	t.Setenv("JWT_SECRET", "test-secret-key")
	auth.JWTSecretForTest()

	pair, err := uc.Login(context.Background(), "user@example.com", "secret1")

	assert.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
}

// --- GetProfile / UpdateProfile / DeleteAccount ---

func TestGetProfileNotFound(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("GetByID", mock.Anything, "missing").Return(model.User{}, repository.ErrUserNotFound)

	_, err := uc.GetProfile(middleware.WithUserID(context.Background(), "missing", ""), "missing")

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

	got, err := uc.GetProfile(middleware.WithUserID(context.Background(), "u1", ""), "u1")

	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestUpdateProfileSuccess(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("Update", mock.Anything, "u1", mock.Anything).Return(nil)

	err := uc.UpdateProfile(middleware.WithUserID(context.Background(), "u1", ""), "u1", model.User{Username: "bob2"})
	assert.NoError(t, err)
}

func TestUpdateProfileFailure(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("Update", mock.Anything, "u1", mock.Anything).Return(errors.New("db error"))

	err := uc.UpdateProfile(middleware.WithUserID(context.Background(), "u1", ""), "u1", model.User{Username: "bob2"})
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 500, appErr.Code)
}

func TestDeleteAccountSuccess(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("Delete", mock.Anything, "u1").Return(nil)

	err := uc.DeleteAccount(middleware.WithUserID(context.Background(), "u1", ""), "u1")
	assert.NoError(t, err)
}

func TestDeleteAccountFailure(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("Delete", mock.Anything, "u1").Return(errors.New("db error"))

	err := uc.DeleteAccount(middleware.WithUserID(context.Background(), "u1", ""), "u1")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 500, appErr.Code)
}

func TestUserAccessRejectsOtherUser(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)
	ctx := middleware.WithUserID(context.Background(), "u1", "")

	_, err := uc.GetProfile(ctx, "u2")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 403, appErr.Code)
	repo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
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
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)

	t.Setenv("JWT_SECRET", "test-secret-key")
	auth.JWTSecretForTest()

	pair, err := uc.OAuthLogin(context.Background(), "google", "fake-id-token")
	assert.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	repo.AssertCalled(t, "UpsertByExternal", mock.Anything, mock.Anything)
	repo.AssertCalled(t, "CreateRefreshToken", mock.Anything, mock.Anything)
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

// --- RefreshTokens ---

func TestRefreshTokensSuccess(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("GetRefreshTokenByHash", mock.Anything, "", mock.Anything).Return(model.RefreshToken{
		UserID:    "u1",
		TokenHash: "hash1",
		ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
		Revoked:   false,
	}, nil)
	repo.On("GetByID", mock.Anything, "u1").Return(model.User{ID: "u1", Role: model.RoleAdmin}, nil)
	repo.On("RevokeRefreshToken", mock.Anything, "u1", "hash1").Return(nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.Anything).Return(nil)

	t.Setenv("JWT_SECRET", "test-secret-key")
	auth.JWTSecretForTest()

	pair, err := uc.RefreshTokens(context.Background(), "raw-token-1")
	assert.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	_, role, err := auth.ValidateToken(pair.AccessToken)
	assert.NoError(t, err)
	assert.Equal(t, model.RoleAdmin, role)
}

func TestRefreshTokensEmptyToken(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	_, err := uc.RefreshTokens(context.Background(), "")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 401, appErr.Code)
}

func TestRefreshTokensNotFound(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)
	repo.On("GetRefreshTokenByHash", mock.Anything, "", mock.Anything).Return(model.RefreshToken{}, repository.ErrUserNotFound)

	_, err := uc.RefreshTokens(context.Background(), "missing")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 401, appErr.Code)
}

func TestRefreshTokensReuseDetected(t *testing.T) {
	repo := new(mockUserRepo)
	uc := newUserUsecase(repo)

	repo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)
	repo.On("GetRefreshTokenByHash", mock.Anything, "", mock.Anything).Return(model.RefreshToken{
		UserID:    "u1",
		TokenHash: "hash1",
		Revoked:   true,
	}, nil)
	repo.On("RevokeAllRefreshTokens", mock.Anything, "u1").Return(nil)

	_, err := uc.RefreshTokens(context.Background(), "reused-token")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 401, appErr.Code)
	repo.AssertCalled(t, "RevokeAllRefreshTokens", mock.Anything, "u1")
}
