package usecase

import (
	"context"
	"time"

	"beer-review-app/internal/user/model"
	"beer-review-app/internal/user/repository"
	"beer-review-app/pkg/auth"
	"beer-review-app/pkg/errors"

	"github.com/google/uuid"
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
	// Check if email already exists
	_, err := u.repo.GetByEmail(ctx, user.Email)
	if err == nil {
		return errors.NewAppError(400, "email already registered", nil)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.NewAppError(500, "failed to hash password", err)
	}

	// Prepare user data
	user.ID = uuid.New().String()
	user.Password = string(hashedPassword)
	user.Created = time.Now().UTC().Format(time.RFC3339)

	if err := u.repo.Create(ctx, user); err != nil {
		return errors.NewAppError(500, "failed to create user", err)
	}

	return nil
}

func (u *userUsecase) Login(ctx context.Context, email, password string) (string, error) {
	user, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", errors.NewAppError(401, "invalid credentials", nil)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.NewAppError(401, "invalid credentials", nil)
	}

	// Generate JWT token
	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		return "", errors.NewAppError(500, "failed to generate token", err)
	}

	return token, nil
}

func (u *userUsecase) GetProfile(ctx context.Context, id string) (model.User, error) {
	user, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return model.User{}, errors.NewAppError(404, "user not found", err)
	}
	return user, nil
}

func (u *userUsecase) UpdateProfile(ctx context.Context, id string, user model.User) error {
	if err := u.repo.Update(ctx, id, user); err != nil {
		return errors.NewAppError(500, "failed to update profile", err)
	}
	return nil
}

func (u *userUsecase) DeleteAccount(ctx context.Context, id string) error {
	if err := u.repo.Delete(ctx, id); err != nil {
		return errors.NewAppError(500, "failed to delete account", err)
	}
	return nil
}
