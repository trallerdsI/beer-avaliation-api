package usecase

import (
	"context"
	"testing"

	"beer-review-app/internal/beer/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGetAll tests the GetAll method
func TestGetAll(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil)

	beers := []model.Beer{{ID: "1", Name: "Beer1"}, {ID: "2", Name: "Beer2"}}

	mockRepo.On("GetAll", mock.Anything).Return(beers, nil)

	result, err := usecase.GetAll(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, beers, result)
	mockRepo.AssertExpectations(t)
}

// TestCreate tests the Create method
func TestCreate(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil)

	beer := model.Beer{ID: "1", Name: "Beer1"}

	mockRepo.On("Create", mock.Anything, beer).Return(nil)

	err := usecase.Create(context.Background(), beer)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestGetByID tests the GetByID method
func TestGetByID(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil)

	beer := model.Beer{ID: "1", Name: "Beer1"}

	mockRepo.On("GetByID", mock.Anything, "1").Return(beer, nil)

	result, err := usecase.GetByID(context.Background(), "1")

	assert.NoError(t, err)
	assert.Equal(t, beer, result)
	mockRepo.AssertExpectations(t)
}

// TestUpdate tests the Update method
func TestUpdate(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil)

	beer := model.Beer{ID: "1", Name: "Updated Beer"}

	mockRepo.On("Update", mock.Anything, "1", beer).Return(nil)

	err := usecase.Update(context.Background(), "1", beer)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestDelete tests the Delete method
func TestDelete(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil)

	mockRepo.On("Delete", mock.Anything, "1").Return(nil)

	err := usecase.Delete(context.Background(), "1")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestAddComment tests the AddComment method
func TestAddComment(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil)
	commentText := "Nice beer!"
	beer := model.Beer{ID: "1", Name: "Beer1", Comments: []model.Comment{}}
	comment := model.Comment{ID: "c1", Text: commentText}

	mockRepo.On("GetByID", mock.Anything, "1").Return(beer, nil)
	mockRepo.On("Update", mock.Anything, "1", mock.Anything).Return(nil)

	err := usecase.AddComment(context.Background(), "1", comment)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestDeleteComment tests the DeleteComment method
func TestDeleteComment(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil)

	beer := model.Beer{ID: "1", Name: "Beer1", Comments: []model.Comment{{ID: "c1"}}}

	mockRepo.On("GetByID", mock.Anything, "1").Return(beer, nil)
	mockRepo.On("Update", mock.Anything, "1", mock.Anything).Return(nil)

	err := usecase.DeleteComment(context.Background(), "1", "c1")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestLikeComment tests the LikeComment method
func TestLikeComment(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil)

	beer := model.Beer{ID: "1", Name: "Beer1", Comments: []model.Comment{{ID: "c1", Likes: 0}}}
	deviceID := "device1"

	mockRepo.On("GetByID", mock.Anything, "1").Return(beer, nil)
	mockRepo.On("Update", mock.Anything, "1", mock.Anything).Return(nil)

	err := usecase.LikeComment(context.Background(), "1", "c1", deviceID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// TestSearchBeers tests the SearchBeers method
func TestSearchBeers(t *testing.T) {
	mockRepo := new(MockBeerRepository)
	usecase := NewBeerUsecase(mockRepo, nil)

	filters := model.BeerFilters{Query: "Beer", Page: 1, PageSize: 10}
	beers := []model.Beer{{ID: "1", Name: "Beer1"}}
	total := 1

	mockRepo.On("SearchBeers", mock.Anything, filters).Return(beers, total, nil)

	result, totalCount, err := usecase.SearchBeers(context.Background(), filters)

	assert.NoError(t, err)
	assert.Equal(t, beers, result)
	assert.Equal(t, total, totalCount)
	mockRepo.AssertExpectations(t)
}
