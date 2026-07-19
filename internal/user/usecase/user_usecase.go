package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/repository"
	"beer-review-app/pkg/auth"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/uuid"

	"golang.org/x/crypto/bcrypt"
)

type UserUsecase interface {
	Register(ctx context.Context, user model.User) error
	Login(ctx context.Context, email, password string) (string, error) // Returns JWT token
	OAuthLogin(ctx context.Context, provider, idToken string) (string, error)
	GetProfile(ctx context.Context, id string) (model.User, error)
	UpdateProfile(ctx context.Context, id string, user model.User) error
	DeleteAccount(ctx context.Context, id string) error
	SeedAdmin(ctx context.Context) error
}

type userUsecase struct {
	repo repository.UserRepository
}

func NewUserUsecase(repo repository.UserRepository) UserUsecase {
	return &userUsecase{repo: repo}
}

func (u *userUsecase) Register(ctx context.Context, user model.User) error {
	slog.InfoContext(ctx, "register attempt", "username", user.Username)

	// Check if email already exists
	_, err := u.repo.GetByEmail(ctx, user.Email)
	if err == nil {
		slog.WarnContext(ctx, "registration failed: email already registered", "username", user.Username)
		return errors.NewAppError(400, "email already registered", nil)
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
	if user.Role == "" {
		user.Role = model.RoleUser
	}

	// Create user in the repository (user.ID will be auto-generated)
	if err := u.repo.Create(ctx, user); err != nil {
		slog.ErrorContext(ctx, "failed to create user", "err", err)
		return errors.NewAppError(500, "failed to create user", err)
	}

	slog.InfoContext(ctx, "user registered", "username", user.Username)
	return nil
}

func (u *userUsecase) Login(ctx context.Context, email, password string) (string, error) {
	slog.InfoContext(ctx, "login attempt")

	// Defesa: limita o tamanho da senha antes do bcrypt.
	if len(password) > 72 {
		slog.WarnContext(ctx, "login failed: password too long")
		return "", errors.NewAppError(401, "invalid credentials", nil)
	}

	// Get user by email
	user, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		slog.WarnContext(ctx, "login failed: user not found")
		return "", errors.NewAppError(401, "invalid credentials", nil)
	}

	// Compare passwords
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		slog.WarnContext(ctx, "login failed: invalid password")
		return "", errors.NewAppError(401, "invalid credentials", nil)
	}

	// Generate JWT token
	token, err := auth.GenerateToken(user.ID, user.Role)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate token", "err", err)
		return "", errors.NewAppError(500, "failed to generate token", err)
	}

	slog.InfoContext(ctx, "user logged in", "user_id", user.ID)
	return token, nil
}

// OAuthLogin valida um id_token OIDC (RFC 6749/OpenID Connect) de um
// provedor configurado (Google/Apple), faz upsert do utilizador em beerUsers
// ligado por (provider, external_sub) e devolve o nosso JWT HS256 de sessão.
// O resto da API continua a aceitar apenas o nosso token via middleware Auth.
func (u *userUsecase) OAuthLogin(ctx context.Context, provider, idToken string) (string, error) {
	slog.InfoContext(ctx, "oauth login attempt", "provider", provider)

	verifier := auth.OIDC().Verifier(provider)
	if verifier == nil {
		slog.WarnContext(ctx, "oauth provider não configurado", "provider", provider)
		return "", errors.NewAppError(400, "unsupported provider", nil)
	}

	claims, err := verifier.Verify(ctx, idToken)
	if err != nil {
		slog.WarnContext(ctx, "oauth token inválido", "provider", provider, "err", err)
		return "", errors.NewAppError(401, "invalid id_token", err)
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
		return "", errors.NewAppError(500, "failed to create user", err)
	}

	token, err := auth.GenerateToken(user.ID, user.Role)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate token", "err", err)
		return "", errors.NewAppError(500, "failed to generate token", err)
	}

	slog.InfoContext(ctx, "oauth user logged in", "user_id", user.ID, "provider", provider)
	return token, nil
}

func (u *userUsecase) GetProfile(ctx context.Context, id string) (model.User, error) {
	slog.InfoContext(ctx, "fetch profile", "user_id", id)

	// Get user by ID
	user, err := u.repo.GetByID(ctx, id)
	if err != nil {
		slog.WarnContext(ctx, "profile not found", "user_id", id)
		return model.User{}, errors.NewAppError(404, "user not found", err)
	}

	slog.InfoContext(ctx, "profile fetched", "user_id", id)
	return user, nil
}

func (u *userUsecase) UpdateProfile(ctx context.Context, id string, user model.User) error {
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
	slog.InfoContext(ctx, "delete account attempt", "user_id", id)

	// Delete user account
	if err := u.repo.Delete(ctx, id); err != nil {
		slog.ErrorContext(ctx, "failed to delete account", "user_id", id, "err", err)
		return errors.NewAppError(500, "failed to delete account", err)
	}

	slog.InfoContext(ctx, "account deleted", "user_id", id)
	return nil
}

// SeedAdmin cria um utilizador administrador global no arranque quando
// ADMIN_EMAIL e ADMIN_PASSWORD estão definidos no ambiente. Se o email já
// existir, promove o utilizador existente a admin (idempotente). O seed é
// opcional: em ausência das env vars, nenhum admin é criado automaticamente.
func (u *userUsecase) SeedAdmin(ctx context.Context) error {
	email := os.Getenv("ADMIN_EMAIL")
	password := os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		slog.InfoContext(ctx, "seed admin ignorado: ADMIN_EMAIL/ADMIN_PASSWORD não definidos")
		return nil
	}

	existing, err := u.repo.GetByEmail(ctx, email)
	if err == nil {
		// Já existe: garante que é admin.
		if existing.Role == model.RoleAdmin {
			return nil
		}
		existing.Role = model.RoleAdmin
		if err := u.repo.Update(ctx, existing.ID, existing); err != nil {
			return fmt.Errorf("failed to promote admin: %w", err)
		}
		slog.InfoContext(ctx, "utilizador promovido a admin", "email", email)
		return nil
	}

	// Não existe: cria.
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
	if err := u.repo.Create(ctx, admin); err != nil {
		return fmt.Errorf("failed to create admin: %w", err)
	}
	slog.InfoContext(ctx, "admin global criado", "email", email)
	return nil
}
