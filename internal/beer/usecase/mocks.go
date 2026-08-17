package usecase

import (
	"context"
	"time"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/repository"

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

// GetAdminStats mocks the GetAdminStats method of BeerRepository
func (m *MockBeerRepository) GetAdminStats(ctx context.Context) (*repository.AdminStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.AdminStats), args.Error(1)
}

// GetUserStats mocks the GetUserStats method of BeerRepository
func (m *MockBeerRepository) GetUserStats(ctx context.Context, userID string) (*repository.UserStats, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.UserStats), args.Error(1)
}

// AddMedia mocks the AddMedia method of BeerRepository
func (m *MockBeerRepository) AddMedia(ctx context.Context, id string, item model.MediaItem) ([]model.MediaItem, error) {
	args := m.Called(ctx, id, item)
	return args.Get(0).([]model.MediaItem), args.Error(1)
}

type MockReportRepository struct {
	mock.Mock
}

func (m *MockReportRepository) CreateReport(ctx context.Context, report *model.BeerReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *MockReportRepository) GetReportsByBeerID(ctx context.Context, beerID string, limit, offset int) ([]model.BeerReport, int, error) {
	args := m.Called(ctx, beerID, limit, offset)
	return args.Get(0).([]model.BeerReport), args.Get(1).(int), args.Error(2)
}

func (m *MockReportRepository) GetReports(ctx context.Context, filter model.ReportFilter) ([]model.BeerReport, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]model.BeerReport), args.Get(1).(int), args.Error(2)
}

func (m *MockReportRepository) ResolveReport(ctx context.Context, reportID, status string, resolvedBy *string, resolvedAt time.Time) error {
	args := m.Called(ctx, reportID, status, resolvedBy, resolvedAt)
	return args.Error(0)
}

type MockDeletionRequestRepository struct {
	mock.Mock
}

func (m *MockDeletionRequestRepository) CreateDeletionRequest(ctx context.Context, req *model.BeerDeletionRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockDeletionRequestRepository) GetDeletionRequests(ctx context.Context, filter model.DeletionRequestFilter) ([]model.BeerDeletionRequest, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]model.BeerDeletionRequest), args.Get(1).(int), args.Error(2)
}

func (m *MockDeletionRequestRepository) ResolveDeletionRequest(ctx context.Context, reqID, status string, reviewedBy *string, reviewedAt time.Time) error {
	args := m.Called(ctx, reqID, status, reviewedBy, reviewedAt)
	return args.Error(0)
}
