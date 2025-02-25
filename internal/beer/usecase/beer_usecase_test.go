package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"beer-review-app/internal/beer/model"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockBeerRepository is a mock implementation of the BeerRepository interface for testing purposes.
type MockBeerRepository struct {
	mock.Mock
}

func (m *MockBeerRepository) GetAll(ctx context.Context) ([]model.Beer, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Beer), args.Error(1)
}

func (m *MockBeerRepository) Create(ctx context.Context, beer model.Beer) error {
	args := m.Called(ctx, beer)
	return args.Error(0)
}

func (m *MockBeerRepository) GetByID(ctx context.Context, id string) (model.Beer, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.Beer), args.Error(1)
}

func (m *MockBeerRepository) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]model.Beer), args.Int(1), args.Error(2)
}

func (m *MockBeerRepository) Update(ctx context.Context, id string, beer model.Beer) error {
	args := m.Called(ctx, id, beer)
	return args.Error(0)
}

func (m *MockBeerRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockBeerRepository) AddComment(ctx context.Context, id string, comment model.Comment) error {
	args := m.Called(ctx, id, comment)
	return args.Error(0)
}

func (m *MockBeerRepository) DeleteComment(ctx context.Context, id string, commentID string) error {
	args := m.Called(ctx, id, commentID)
	return args.Error(0)
}

func (m *MockBeerRepository) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]model.Beer), args.Int(1), args.Error(2)
}

// MockRedis is a mock implementation of Redis client
type MockRedis struct {
	mock.Mock
	redis.Cmdable
}

func TestBeerUsecase_Create(t *testing.T) {
	repo := new(MockBeerRepository)
	redisClient := &redis.Client{}
	usecase := NewBeerUsecase(repo, redisClient)

	beer := model.Beer{Name: "Test Beer", Style: "IPA"}
	err := usecase.Create(context.Background(), beer)

	assert.NoError(t, err)

	beers, _ := usecase.GetAll(context.Background())
	assert.Len(t, beers, 1)
}

func TestBeerUsecase_GetAll(t *testing.T) {
	repo := new(MockBeerRepository)
	redisClient := &redis.Client{} // Assuming redis.Client is a valid type for the second argument
	usecase := NewBeerUsecase(repo, redisClient)

	usecase.Create(context.Background(), model.Beer{Name: "Test Beer 1", Style: "IPA"})
	usecase.Create(context.Background(), model.Beer{Name: "Test Beer 2", Style: "Stout"})

	beers, _ := usecase.GetAll(context.Background())
	assert.Len(t, beers, 2)
}

func TestGetByIDNotFound(t *testing.T) {
	// Setup mock repository
	mockRepo := new(MockBeerRepository)
	mockRepo.On("GetByID", mock.Anything, "non-existent-id").Return(model.Beer{}, fmt.Errorf("not found"))

	// Assuming NewBeerUsecase can handle a nil redisClient argument
	usecase := NewBeerUsecase(mockRepo, nil)

	_, err := usecase.GetByID(context.Background(), "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetAll(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	mockRedis := new(MockRedis)
	usecase := NewBeerUsecase(mockRepo, mockRedis)

	expectedBeers := []model.Beer{
		{ID: "1", Name: "Test Beer 1"},
		{ID: "2", Name: "Test Beer 2"},
	}

	// Setup mock repository
	mockRepo.On("GetAll", mock.Anything).Return(expectedBeers, nil)

	// Setup mock redis to simulate cache miss
	mockRedis.On("Get", mock.Anything, "all_beers").Return(redis.NewStringResult("", redis.Nil))
	mockRedis.On("Set", mock.Anything, "all_beers", mock.Anything, time.Duration(0)).Return(redis.NewStatusResult("OK", nil))

	// Test
	beers, err := usecase.GetAll(context.Background())

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedBeers, beers)
	mockRepo.AssertExpectations(t)
	mockRedis.AssertExpectations(t)
}

func TestCreate(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	mockRedis := new(MockRedis)
	usecase := NewBeerUsecase(mockRepo, mockRedis)

	beer := model.Beer{
		Name:  "New Beer",
		Style: "IPA",
	}

	// Setup expectations
	mockRepo.On("Create", mock.Anything, beer).Return(nil)
	mockRedis.On("Del", mock.Anything, []string{"all_beers"}).Return(redis.NewIntResult(1, nil))

	// Test
	err := usecase.Create(context.Background(), beer)

	// Assert
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockRedis.AssertExpectations(t)
}

func TestGetByID(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	mockRedis := new(MockRedis)
	usecase := NewBeerUsecase(mockRepo, mockRedis)

	expectedBeer := model.Beer{
		ID:   "1",
		Name: "Test Beer",
	}

	// Setup mock
	mockRepo.On("GetByID", mock.Anything, "1").Return(expectedBeer, nil)

	// Test
	beer, err := usecase.GetByID(context.Background(), "1")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedBeer, beer)
	mockRepo.AssertExpectations(t)
}

func TestGetPaginated(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	mockRedis := new(MockRedis)
	usecase := NewBeerUsecase(mockRepo, mockRedis)

	expectedBeers := []model.Beer{
		{ID: "1", Name: "Test Beer 1"},
		{ID: "2", Name: "Test Beer 2"},
	}

	// Setup mock
	mockRepo.On("GetPaginated", mock.Anything, 1, 10).Return(expectedBeers, 2, nil)

	// Test
	beers, total, err := usecase.GetPaginated(context.Background(), 1, 10)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedBeers, beers)
	assert.Equal(t, 2, total)
	mockRepo.AssertExpectations(t)
}

func TestSearchBeers(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	mockRedis := new(MockRedis)
	usecase := NewBeerUsecase(mockRepo, mockRedis)

	filters := model.BeerFilters{
		Query:      "IPA",
		Style:      "American",
		MinAlcohol: &[]float64{5.0}[0],
		MaxAlcohol: &[]float64{7.0}[0],
		Page:       1,
		PageSize:   10,
	}

    alc := 6.0

	expectedBeers := []model.Beer{
		{ID: "1", Name: "Test IPA", Style: "American", Alcohol: &alc},
	}

	// Setup mock repository
	mockRepo.On("SearchBeers", mock.Anything, filters).Return(expectedBeers, 1, nil)

	// Setup mock redis for cache miss
	cacheKey := fmt.Sprintf("search:%s:%s:%v:%v:%s:%d:%d",
		filters.Query, filters.Style, filters.MinAlcohol, filters.MaxAlcohol,
		filters.Taste, filters.Page, filters.PageSize,
	)
	mockRedis.On("Get", mock.Anything, cacheKey).Return(redis.NewStringResult("", redis.Nil))
	mockRedis.On("Set", mock.Anything, cacheKey, mock.Anything, time.Hour).Return(redis.NewStatusResult("OK", nil))

	// Test
	beers, total, err := usecase.SearchBeers(context.Background(), filters)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedBeers, beers)
	assert.Equal(t, 1, total)
	mockRepo.AssertExpectations(t)
	mockRedis.AssertExpectations(t)
}
