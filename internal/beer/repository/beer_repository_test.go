package repository

import (
	"context"
	"testing"

	"beer-review-app/internal/beer/model"
)

func TestInMemoryBeerRepository_Create(t *testing.T) {
	repo := NewInMemoryBeerRepository()

	beer := model.Beer{Name: "Test Beer", Style: "IPA"}
	err := repo.Create(context.Background(), beer)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	beers, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(beers) != 1 {
		t.Fatalf("expected 1 beer, got %d", len(beers))
	}
}

func TestInMemoryBeerRepository_GetAll(t *testing.T) {
	repo := NewInMemoryBeerRepository()
	repo.Create(context.Background(), model.Beer{Name: "Test Beer", Style: "IPA"})

	beers, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(beers) != 1 {
		t.Fatalf("expected 1 beer, got %d", len(beers))
	}
}
