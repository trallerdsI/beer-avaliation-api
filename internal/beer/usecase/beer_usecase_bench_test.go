package usecase

import (
	"context"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/repository"
	"beer-review-app/pkg/moderation"

	"github.com/stretchr/testify/mock"
)

func BenchmarkSearchBeers_ByQuery(b *testing.B) {
	mockRepo := new(repository.MockBeerRepository)
	mockRepo.On("SearchBeers", mock.Anything, mock.Anything).Return([]model.Beer{}, 0, false, nil)
	uc := NewBeerUsecase(mockRepo, moderation.NewNoopModerator(), nil)
	filters := model.BeerFilters{Query: "IPA", Page: 1, PageSize: 20}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = uc.SearchBeers(context.Background(), filters)
	}
}

func BenchmarkSearchBeers_ByStyle(b *testing.B) {
	mockRepo := new(repository.MockBeerRepository)
	mockRepo.On("SearchBeers", mock.Anything, mock.Anything).Return([]model.Beer{}, 0, false, nil)
	uc := NewBeerUsecase(mockRepo, moderation.NewNoopModerator(), nil)
	filters := model.BeerFilters{Style: "Ale", Page: 1, PageSize: 20}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = uc.SearchBeers(context.Background(), filters)
	}
}

func BenchmarkSearchBeers_FuzzyMatch(b *testing.B) {
	mockRepo := new(repository.MockBeerRepository)
	mockRepo.On("SearchBeers", mock.Anything, mock.Anything).Return([]model.Beer{}, 0, true, nil)
	uc := NewBeerUsecase(mockRepo, moderation.NewNoopModerator(), nil)
	fuzzy := true
	filters := model.BeerFilters{Query: "ip", Fuzzy: &fuzzy, Page: 1, PageSize: 20}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = uc.SearchBeers(context.Background(), filters)
	}
}
