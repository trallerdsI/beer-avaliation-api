package usecase

import (
	"context"

	"beer-review-app/internal/beer/model"

	"github.com/stretchr/testify/mock"
)

type MockBeerRepository struct {
	mock.Mock
}

// GetAll mocks the GetAll method of BeerRepository
func (m *MockBeerRepository) GetAll(ctx context.Context) ([]model.Beer, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Beer), args.Error(1)
}

// Create mocks the Create method of BeerRepository
func (m *MockBeerRepository) Create(ctx context.Context, beer *model.Beer) error {
	args := m.Called(ctx, beer)
	return args.Error(0)
}

// GetPaginated mocks the GetPaginated method of BeerRepository
func (m *MockBeerRepository) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]model.Beer), args.Get(1).(int), args.Error(2)
}

// GetByID mocks the GetByID method of BeerRepository
func (m *MockBeerRepository) GetByID(ctx context.Context, id string) (model.Beer, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.Beer), args.Error(1)
}

// Update mocks the Update method of BeerRepository
func (m *MockBeerRepository) Update(ctx context.Context, id string, beer model.Beer) error {
	args := m.Called(ctx, id, beer)
	return args.Error(0)
}

// Delete mocks the Delete method of BeerRepository
func (m *MockBeerRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// AddComment mocks the AddComment method of BeerRepository
func (m *MockBeerRepository) AddComment(ctx context.Context, id string, comment model.Comment) error {
	args := m.Called(ctx, id, comment)
	return args.Error(0)
}

// DeleteComment mocks the DeleteComment method of BeerRepository
func (m *MockBeerRepository) DeleteComment(ctx context.Context, id string, commentID string) error {
	args := m.Called(ctx, id, commentID)
	return args.Error(0)
}

// SearchBeers mocks the SearchBeers method of BeerRepository
func (m *MockBeerRepository) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]model.Beer), args.Get(1).(int), args.Error(2)
}
