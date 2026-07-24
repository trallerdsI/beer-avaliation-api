package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"beer-review-app/internal/beer/model"
)

func TestPostgresBeerRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definida; pulando teste de integração")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("falha ao conectar DB de teste: %v", err)
	}
	defer db.Close()

	repo, err := NewPostgresBeerRepository(db)
	if err != nil {
		t.Fatalf("falha ao criar repo: %v", err)
	}

	ctx := context.Background()
	beer := &model.Beer{
		ID:       "test-beer-1",
		Name:     "Test Beer",
		Style:    string(model.FlavorBitter),
		Alcohol:  ptrFloat64(5.0),
		Comments: []model.Comment{},
	}

	if err := repo.Create(ctx, beer); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "test-beer-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Test Beer" {
		t.Fatalf("Name: got %q, want Test Beer", got.Name)
	}

	beer.Style = string(model.FlavorSweet)
	if err := repo.Update(ctx, "test-beer-1", *beer); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ = repo.GetByID(ctx, "test-beer-1")
	if got.Style != string(model.FlavorSweet) {
		t.Fatalf("Style after update: got %q, want %q", got.Style, model.FlavorSweet)
	}

	if err := repo.Delete(ctx, "test-beer-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.GetByID(ctx, "test-beer-1")
	if err == nil {
		t.Fatal("expected 404 after delete")
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}
