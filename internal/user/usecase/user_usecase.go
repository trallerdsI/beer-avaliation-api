package usecase

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/repository"
	"beer-review-app/pkg/auth"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/uuid"

	"golang.org/x/crypto/bcrypt"
)

type UserUsecase interface {
	Register(ctx context.Context, user model.User) error
	Login(ctx context.Context, email, password string) (auth.TokenPair, error)
	OAuthLogin(ctx context.Context, provider, idToken string) (auth.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken string) (auth.TokenPair, error)
	GetProfile(ctx context.Context, id string) (model.User, error)
	UpdateProfile(ctx context.Context, id string, user model.User) error
	DeleteAccount(ctx context.Context, id string) error
	SeedAdmin(ctx context.Context) error
	SubscribePush(ctx context.Context, userID string, sub model.PushSubscription) error
	UnsubscribePush(ctx context.Context, userID, endpoint string) error
	ListPushSubscriptions(ctx context.Context, userID string) ([]model.PushSubscription, error)
}

type userUsecase struct {
	repo repository.UserRepository
}

func NewUserUsecase(repo repository.UserRepository) UserUsecase {
	return &userUsecase{repo: repo}
}

func (u *userUsecase) unavailable() error {
	if u.repo == nil {
		return errors.NewUnavailableError()
	}
	return nil
}

func (u *userUsecase) Register(ctx context.Context, user model.User) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	slog.InfoContext(ctx, "register attempt", "username", user.Username)

	// Check if email already exists
	_, err := u.repo.GetByEmail(ctx, user.Email)
	if err == nil {
		slog.WarnContext(ctx, "registration failed: email already registered", "username", user.Username)
		return errors.NewAppError(400, "email already registered", nil)
	}
	// Erros não-relacionados a "não encontrado" são falhas de infraestrutura
	// e devem propagar como 500. Sem isto, race conditions e indisponibilidade
	// transitória do banco seriam silenciosamente engolidas, amplificando DoS.
	if !stderrors.Is(err, repository.ErrUserNotFound) {
		slog.ErrorContext(ctx, "failed to lookup user by email", "err", err)
		return errors.NewAppError(500, "failed to lookup user", err)
	}

	// Defesa: limita o tamanho da senha antes do bcrypt para evitar DoS de CPU
	// (custo do bcrypt cresce com o tamanho da entrada). 72 bytes é o teto do bcrypt.
	if len(user.Password) > 72 {
		slog.ErrorContext(ctx, "password exceeds bcrypt limit")
		return errors.NewAppError(400, "password too long", nil)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "failed to hash password", "err", err)
		return errors.NewAppError(500, "failed to hash password", err)
	}

	// Prepare user data. Registos normais são always role "user"; o papel de
	// admin é atribuído apenas via seed de arranque (ADMIN_EMAIL/ADMIN_PASSWORD).
	user.Password = string(hashedPassword)
	user.Created = time.Now().UTC().Format(time.RFC3339)
	user.Role = model.RoleUser

	// Create user in the repository (user.ID will be auto-generated)
	if err := u.repo.Create(ctx, user); err != nil {
		slog.ErrorContext(ctx, "failed to create user", "err", err)
		return errors.NewAppError(500, "failed to create user", err)
	}

	slog.InfoContext(ctx, "user registered", "username", user.Username)
	return nil
}

func (u *userUsecase) Login(ctx context.Context, email, password string) (auth.TokenPair, error) {
	if err := u.unavailable(); err != nil {
		return auth.TokenPair{}, err
	}
	slog.InfoContext(ctx, "login attempt")

	if len(password) > 72 {
		slog.WarnContext(ctx, "login failed: password too long")
		return auth.TokenPair{}, errors.NewAppError(401, "invalid credentials", nil)
	}

	user, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		// Distinguir "não encontrado" (401 genérico, sem vazar existência)
		// de falha de infraestrutura (500 com log).
		if stderrors.Is(err, repository.ErrUserNotFound) {
			slog.WarnContext(ctx, "login failed: user not found")
		} else {
			slog.ErrorContext(ctx, "login failed: database error", "err", err)
		}
		return auth.TokenPair{}, errors.NewAppError(401, "invalid credentials", nil)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		slog.WarnContext(ctx, "login failed: invalid password")
		return auth.TokenPair{}, errors.NewAppError(401, "invalid credentials", nil)
	}

	return u.issueTokenPair(ctx, user.ID, user.Role, u.repo)
}

// OAuthLogin valida um id_token OIDC (RFC 6749/OpenID Connect) de um
// provedor configurado (Google/Apple), faz upsert do utilizador em beerUsers
// ligado por (provider, external_sub) e devolve o nosso JWT HS256 de sessão.
// O resto da API continua a aceitar apenas o nosso token via middleware Auth.
func (u *userUsecase) OAuthLogin(ctx context.Context, provider, idToken string) (auth.TokenPair, error) {
	if err := u.unavailable(); err != nil {
		return auth.TokenPair{}, err
	}
	slog.InfoContext(ctx, "oauth login attempt", "provider", provider)

	verifier := auth.OIDC().Verifier(provider)
	if verifier == nil {
		slog.WarnContext(ctx, "oauth provider não configurado", "provider", provider)
		return auth.TokenPair{}, errors.NewAppError(400, "unsupported provider", nil)
	}

	claims, err := verifier.Verify(ctx, idToken)
	if err != nil {
		slog.WarnContext(ctx, "oauth token inválido", "provider", provider, "err", err)
		return auth.TokenPair{}, errors.NewAppError(401, "invalid id_token", err)
	}

	user, err := u.repo.UpsertByExternal(ctx, model.User{
		ID:          uuid.MustNewV7(),
		Username:    claims.Username,
		Email:       claims.Email,
		Role:        model.RoleUser,
		Provider:    provider,
		ExternalSub: claims.Sub,
		Created:     time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to upsert oauth user", "err", err)
		return auth.TokenPair{}, errors.NewAppError(500, "failed to create user", err)
	}

	pair, err := u.issueTokenPair(ctx, user.ID, user.Role, u.repo)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate token pair", "err", err)
		return auth.TokenPair{}, errors.NewAppError(500, "failed to generate token", err)
	}

	slog.InfoContext(ctx, "oauth user logged in", "user_id", user.ID, "provider", provider)
	return pair, nil
}

func (u *userUsecase) issueTokenPair(ctx context.Context, userID, role string, repo repository.UserRepository) (auth.TokenPair, error) {
	accessToken, err := auth.GenerateToken(userID, role)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate access token", "err", err)
		return auth.TokenPair{}, errors.NewAppError(500, "failed to generate token", err)
	}

	rawRefreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate refresh token", "err", err)
		return auth.TokenPair{}, errors.NewAppError(500, "failed to generate refresh token", err)
	}

	tokenHash := auth.HashToken(rawRefreshToken)
	now := time.Now().UTC()
	refreshToken := model.RefreshToken{
		ID:        uuid.MustNewV7(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(auth.RefreshTokenTTL()).Format(time.RFC3339),
		Revoked:   false,
		CreatedAt: now.Format(time.RFC3339),
	}

	if err := repo.CreateRefreshToken(ctx, refreshToken); err != nil {
		slog.ErrorContext(ctx, "failed to persist refresh token", "err", err)
		return auth.TokenPair{}, errors.NewAppError(500, "failed to persist refresh token", err)
	}

	return auth.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
	}, nil
}

func (u *userUsecase) RefreshTokens(ctx context.Context, refreshToken string) (auth.TokenPair, error) {
	if err := u.unavailable(); err != nil {
		return auth.TokenPair{}, err
	}
	if refreshToken == "" {
		return auth.TokenPair{}, errors.NewAppError(401, "invalid refresh token", nil)
	}

	tokenHash := auth.HashToken(refreshToken)

	stored, err := u.repo.GetRefreshTokenByHash(ctx, "", tokenHash)
	if err != nil {
		if stderrors.Is(err, repository.ErrUserNotFound) {
			slog.WarnContext(ctx, "refresh token not found")
			return auth.TokenPair{}, errors.NewAppError(401, "invalid refresh token", nil)
		}
		slog.ErrorContext(ctx, "refresh token lookup failed", "err", err)
		return auth.TokenPair{}, errors.NewAppError(500, "failed to lookup refresh token", err)
	}

	if stored.Revoked {
		slog.WarnContext(ctx, "refresh token reuse detected", "user_id", stored.UserID)
		if revokeErr := u.repo.RevokeAllRefreshTokens(ctx, stored.UserID); revokeErr != nil {
			slog.ErrorContext(ctx, "failed to revoke all refresh tokens on reuse", "err", revokeErr)
		}
		return auth.TokenPair{}, errors.NewAppError(401, "invalid refresh token", nil)
	}

	if stored.IsExpired() {
		slog.WarnContext(ctx, "refresh token expired", "user_id", stored.UserID)
		return auth.TokenPair{}, errors.NewAppError(401, "invalid refresh token", nil)
	}

	user, err := u.repo.GetByID(ctx, stored.UserID)
	if err != nil {
		if stderrors.Is(err, repository.ErrUserNotFound) {
			return auth.TokenPair{}, errors.NewAppError(401, "invalid refresh token", nil)
		}
		return auth.TokenPair{}, errors.NewAppError(500, "failed to lookup user", err)
	}

	if err := u.repo.RevokeRefreshToken(ctx, stored.UserID, stored.TokenHash); err != nil {
		slog.ErrorContext(ctx, "failed to revoke refresh token", "err", err)
		return auth.TokenPair{}, errors.NewAppError(500, "failed to revoke refresh token", err)
	}

	pair, err := u.issueTokenPair(ctx, stored.UserID, user.Role, u.repo)
	if err != nil {
		slog.ErrorContext(ctx, "failed to issue new token pair", "err", err)
		return auth.TokenPair{}, err
	}

	slog.InfoContext(ctx, "refresh token rotated", "user_id", stored.UserID)
	return pair, nil
}

func (u *userUsecase) GetProfile(ctx context.Context, id string) (model.User, error) {
	if err := u.unavailable(); err != nil {
		return model.User{}, err
	}
	if err := requireUserAccess(ctx, id); err != nil {
		return model.User{}, err
	}
	slog.InfoContext(ctx, "fetch profile", "user_id", id)

	// Get user by ID
	user, err := u.repo.GetByID(ctx, id)
	if err != nil {
		if stderrors.Is(err, repository.ErrUserNotFound) {
			slog.WarnContext(ctx, "profile not found", "user_id", id)
			return model.User{}, errors.NewAppError(404, "user not found", err)
		}
		slog.ErrorContext(ctx, "profile lookup failed", "user_id", id, "err", err)
		return model.User{}, errors.NewAppError(500, "failed to get profile", err)
	}

	slog.InfoContext(ctx, "profile fetched", "user_id", id)
	return user, nil
}

func (u *userUsecase) UpdateProfile(ctx context.Context, id string, user model.User) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	if err := requireUserAccess(ctx, id); err != nil {
		return err
	}
	slog.InfoContext(ctx, "update profile attempt", "user_id", id)

	// Update user profile
	if err := u.repo.Update(ctx, id, user); err != nil {
		slog.ErrorContext(ctx, "failed to update profile", "user_id", id, "err", err)
		return errors.NewAppError(500, "failed to update profile", err)
	}

	slog.InfoContext(ctx, "profile updated", "user_id", id)
	return nil
}

func (u *userUsecase) DeleteAccount(ctx context.Context, id string) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	if err := requireUserAccess(ctx, id); err != nil {
		return err
	}
	slog.InfoContext(ctx, "delete account attempt", "user_id", id)

	// Delete user account
	if err := u.repo.Delete(ctx, id); err != nil {
		slog.ErrorContext(ctx, "failed to delete account", "user_id", id, "err", err)
		return errors.NewAppError(500, "failed to delete account", err)
	}

	slog.InfoContext(ctx, "account deleted", "user_id", id)
	return nil
}

func (u *userUsecase) SubscribePush(ctx context.Context, userID string, sub model.PushSubscription) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	if err := requireUserAccess(ctx, userID); err != nil {
		return err
	}
	slog.InfoContext(ctx, "subscribe push", "user_id", userID, "endpoint", sub.Endpoint)
	sub.UserID = userID
	if err := u.repo.CreatePushSubscription(ctx, sub); err != nil {
		slog.ErrorContext(ctx, "failed to subscribe push", "err", err)
		return errors.NewAppError(500, "failed to subscribe push", err)
	}
	return nil
}

func (u *userUsecase) UnsubscribePush(ctx context.Context, userID, endpoint string) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	if err := requireUserAccess(ctx, userID); err != nil {
		return err
	}
	slog.InfoContext(ctx, "unsubscribe push", "user_id", userID, "endpoint", endpoint)
	if err := u.repo.DeletePushSubscriptionByEndpoint(ctx, userID, endpoint); err != nil {
		slog.ErrorContext(ctx, "failed to unsubscribe push", "err", err)
		return errors.NewAppError(500, "failed to unsubscribe push", err)
	}
	return nil
}

func (u *userUsecase) ListPushSubscriptions(ctx context.Context, userID string) ([]model.PushSubscription, error) {
	if err := u.unavailable(); err != nil {
		return nil, err
	}
	if err := requireUserAccess(ctx, userID); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "list push subscriptions", "user_id", userID)
	subs, err := u.repo.ListPushSubscriptions(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list push subscriptions", "err", err)
		return nil, errors.NewAppError(500, "failed to list push subscriptions", err)
	}
	return subs, nil
}

func requireUserAccess(ctx context.Context, userID string) error {
	if middleware.IsAdmin(ctx) {
		return nil
	}
	requesterID, ok := middleware.UserIDFromContext(ctx)
	if !ok || requesterID != userID {
		return errors.NewAppError(403, "you are not allowed to access this user", nil)
	}
	return nil
}

// SeedAdmin cria um utilizador administrador global no arranque quando
// ADMIN_EMAIL e ADMIN_PASSWORD estão definidos no ambiente. Se o email já
// existir, promove o utilizador existente a admin (idempotente). O seed é
// opcional: em ausência das env vars, nenhum admin é criado automaticamente.
func (u *userUsecase) SeedAdmin(ctx context.Context) error {
	if err := u.unavailable(); err != nil {
		return err
	}
	email := os.Getenv("ADMIN_EMAIL")
	password := os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		slog.InfoContext(ctx, "seed admin ignorado: ADMIN_EMAIL/ADMIN_PASSWORD não definidos")
		return nil
	}

	return u.repo.ExecInTx(ctx, func(ctx context.Context, txRepo repository.UserRepository) error {
		existing, err := txRepo.GetByEmail(ctx, email)
		if err == nil {
			if existing.Role == model.RoleAdmin {
				return nil
			}
			existing.Role = model.RoleAdmin
			if err := txRepo.Update(ctx, existing.ID, existing); err != nil {
				return fmt.Errorf("failed to promote admin: %w", err)
			}
			slog.InfoContext(ctx, "utilizador promovido a admin", "email", email)
			return nil
		}
		// Qualquer erro que não seja "não encontrado" aborta a transação.
		if !stderrors.Is(err, repository.ErrUserNotFound) {
			return fmt.Errorf("seed admin lookup: %w", err)
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash admin password: %w", err)
		}
		admin := model.User{
			ID:       uuid.MustNewV7(),
			Username: "admin",
			Email:    email,
			Password: string(hashed),
			Role:     model.RoleAdmin,
			Created:  time.Now().UTC().Format(time.RFC3339),
		}
		if err := txRepo.Create(ctx, admin); err != nil {
			return fmt.Errorf("failed to create admin: %w", err)
		}
		slog.InfoContext(ctx, "admin global criado", "email", email)
		return nil
	})
}
