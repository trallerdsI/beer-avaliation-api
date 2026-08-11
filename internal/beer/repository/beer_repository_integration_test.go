package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/uuid"
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
	userID := uuid.MustNewV7()
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := db.ExecContext(ctx, `
		INSERT INTO beerUsers (id, username, email, password, role, created)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, "tester", "test@example.com", "hashed", "user", now,
	); err != nil {
		t.Fatalf("create user: %v", err)
	}

	beerID := uuid.MustNewV7()
	beer := &model.Beer{
		ID:        beerID,
		Name:      "Test Beer",
		Style:     string(model.FlavorBitter),
		CreatedBy: userID,
		CreatedAt: now,
		Alcohol:   ptrFloat64(5.0),
		Comments: []model.Comment{
			{ID: uuid.MustNewV7(), Text: "good", Rating: 5, CreatedBy: userID, CreatedAt: now},
			{ID: uuid.MustNewV7(), Text: "bad", Rating: 1, CreatedBy: userID, CreatedAt: now},
		},
	}

	if err := repo.Create(ctx, beer); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, beerID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Test Beer" {
		t.Fatalf("Name: got %q, want Test Beer", got.Name)
	}

	beer.Style = string(model.FlavorSweet)
	if err := repo.Update(ctx, beerID, *beer); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ = repo.GetByID(ctx, beerID)
	if got.Style != string(model.FlavorSweet) {
		t.Fatalf("Style after update: got %q, want %q", got.Style, model.FlavorSweet)
	}

	if err := repo.Delete(ctx, beerID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.GetByID(ctx, beerID)
	if err == nil {
		t.Fatal("expected 404 after delete")
	}
}

func TestPostgresBeerRepositoryIntegrationAggregations(t *testing.T) {
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
	userID := uuid.MustNewV7()
	now := time.Now().UTC().Format(time.RFC3339)
	username := "tester-" + userID[:8]
	email := "test-" + userID[:8] + "@example.com"

	if _, err := db.ExecContext(ctx, `
		INSERT INTO beerUsers (id, username, email, password, role, created)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, username, email, "hashed", "user", now,
	); err != nil {
		t.Fatalf("create user: %v", err)
	}

	beerID := uuid.MustNewV7()
	beer := &model.Beer{
		ID:        beerID,
		Name:      "Agg Beer",
		Style:     string(model.FlavorBitter),
		CreatedBy: userID,
		CreatedAt: now,
		Alcohol:   ptrFloat64(5.0),
		Comments: []model.Comment{
			{ID: uuid.MustNewV7(), Text: "good", Rating: 5, CreatedBy: userID, CreatedAt: now},
			{ID: uuid.MustNewV7(), Text: "bad", Rating: 1, CreatedBy: userID, CreatedAt: now},
		},
	}
	if err := repo.Create(ctx, beer); err != nil {
		t.Fatalf("create beer: %v", err)
	}

	adminStats, err := repo.GetAdminStats(ctx)
	if err != nil {
		t.Fatalf("GetAdminStats: %v", err)
	}
	if adminStats.TotalBeers < 1 {
		t.Fatalf("expected at least 1 beer, got %d", adminStats.TotalBeers)
	}
	if adminStats.TotalComments < 2 {
		t.Fatalf("expected at least 2 comments, got %d", adminStats.TotalComments)
	}

	userStats, err := repo.GetUserStats(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserStats: %v", err)
	}
	if userStats.TotalComments != 2 {
		t.Fatalf("expected 2 comments for user, got %d", userStats.TotalComments)
	}
	if userStats.BeersReviewed != 2 {
		t.Fatalf("expected 2 beers reviewed, got %d", userStats.BeersReviewed)
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}
