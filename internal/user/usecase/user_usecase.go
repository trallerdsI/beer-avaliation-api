package usecase

import (
	"context"
	"log/slog"
	"time"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/repository"
	"beer-review-app/pkg/auth"
	"beer-review-app/pkg/errors"

	"golang.org/x/crypto/bcrypt"
)

type UserUsecase interface {
	Register(ctx context.Context, user model.User) error
	Login(ctx context.Context, email, password string) (string, error) // Returns JWT token
	GetProfile(ctx context.Context, id string) (model.User, error)
	UpdateProfile(ctx context.Context, id string, user model.User) error
	DeleteAccount(ctx context.Context, id string) error
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

	// Prepare user data
	user.Password = string(hashedPassword)
	user.Created = time.Now().UTC().Format(time.RFC3339)

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
	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate token", "err", err)
		return "", errors.NewAppError(500, "failed to generate token", err)
	}

	slog.InfoContext(ctx, "user logged in", "user_id", user.ID)
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
