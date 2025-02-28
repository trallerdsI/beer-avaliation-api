package usecase

import (
	"context"
	"log"
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
	// Log the user registration attempt
	log.Printf("Attempting to register user with email: %s, username: %s", user.Email, user.Username)

	// Check if email already exists
	_, err := u.repo.GetByEmail(ctx, user.Email)
	if err == nil {
		log.Printf("Registration failed: email %s already registered", user.Email)
		return errors.NewAppError(400, "email already registered", nil)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password for user %s: %v", user.Email, err)
		return errors.NewAppError(500, "failed to hash password", err)
	}

	// Prepare user data
	user.Password = string(hashedPassword)
	user.Created = time.Now().UTC().Format(time.RFC3339)

	// Create user in the repository (user.ID will be auto-generated)
	if err := u.repo.Create(ctx, user); err != nil {
		log.Printf("Failed to create user %s: %v", user.Email, err)
		return errors.NewAppError(500, "failed to create user", err)
	}

	// Log successful registration
	log.Printf("User registered successfully with email: %s", user.Email)
	return nil
}

func (u *userUsecase) Login(ctx context.Context, email, password string) (string, error) {
	log.Printf("Attempting to login with email: %s", email)

	// Get user by email
	user, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		log.Printf("Login failed: invalid credentials for email %s", email)
		return "", errors.NewAppError(401, "invalid credentials", nil)
	}

	// Compare passwords
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Printf("Login failed: invalid password for email %s", email)
		return "", errors.NewAppError(401, "invalid credentials", nil)
	}

	// Generate JWT token
	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		log.Printf("Failed to generate token for user %s: %v", email, err)
		return "", errors.NewAppError(500, "failed to generate token", err)
	}

	// Log successful login
	log.Printf("User logged in successfully with email: %s", email)
	return token, nil
}

func (u *userUsecase) GetProfile(ctx context.Context, id string) (model.User, error) {
	log.Printf("Fetching profile for user ID: %s", id)

	// Get user by ID
	user, err := u.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("Failed to find user profile with ID: %s", id)
		return model.User{}, errors.NewAppError(404, "user not found", err)
	}

	log.Printf("User profile fetched successfully for ID: %s", id)
	return user, nil
}

func (u *userUsecase) UpdateProfile(ctx context.Context, id string, user model.User) error {
	log.Printf("Attempting to update profile for user ID: %s", id)

	// Update user profile
	if err := u.repo.Update(ctx, id, user); err != nil {
		log.Printf("Failed to update profile for user ID: %s: %v", id, err)
		return errors.NewAppError(500, "failed to update profile", err)
	}

	log.Printf("User profile updated successfully for ID: %s", id)
	return nil
}

func (u *userUsecase) DeleteAccount(ctx context.Context, id string) error {
	log.Printf("Attempting to delete account for user ID: %s", id)

	// Delete user account
	if err := u.repo.Delete(ctx, id); err != nil {
		log.Printf("Failed to delete account for user ID: %s: %v", id, err)
		return errors.NewAppError(500, "failed to delete account", err)
	}

	log.Printf("User account deleted successfully for ID: %s", id)
	return nil
}
