package usecase

import (
	"context"

	"beer-review-app/internal/user/model"

	"github.com/stretchr/testify/mock"
)

// MockUserUsecase é uma implementação mock do UserUsecase para testes.
type MockUserUsecase struct {
	mock.Mock
}

func (m *MockUserUsecase) Register(ctx context.Context, user model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserUsecase) Login(ctx context.Context, email, password string) (string, error) {
	args := m.Called(ctx, email, password)
	return args.String(0), args.Error(1)
}

func (m *MockUserUsecase) GetProfile(ctx context.Context, id string) (model.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *MockUserUsecase) UpdateProfile(ctx context.Context, id string, user model.User) error {
	args := m.Called(ctx, id, user)
	return args.Error(0)
}

func (m *MockUserUsecase) DeleteAccount(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
