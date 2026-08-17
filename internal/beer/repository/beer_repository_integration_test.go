//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"beer-review-app/internal/beer/model"

	"github.com/stretchr/testify/require"
)

func TestIntegration_BeerRepository_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := startPostgresContainer(t)
	defer db.Close()

	err := applyMigrations(db)
	require.NoError(t, err)

	repo, err := NewPostgresBeerRepository(db)
	require.NoError(t, err)

	beer := model.Beer{
		ID:          "550e8400-e29b-41d4-a716-446655440000",
		Name:        "IPA Teste",
		Style:       "IPA",
		Description: "Cerveja de teste",
		ImageUrl:    "https://example.com/img.jpg",
		Alcohol:     float64Ptr(5.5),
		Taste:       model.FlavorBitter,
		Aroma:       model.AromaCitrus,
		Color:       model.ColorAmber,
		Body:        model.BodyMedium,
		Carbonation: model.CarbonationMedium,
		Finish:      model.FinishDry,
		CreatedBy:   "user-1",
		CreatedAt:   "2024-01-01T00:00:00Z",
		Comments:    []model.Comment{},
	}

	err = repo.Create(ctx, &beer)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, beer.ID)
	require.NoError(t, err)
	require.Equal(t, beer.Name, got.Name)
	require.Equal(t, "user-1", got.CreatedBy)

	all, err := repo.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)

	page, total, err := repo.GetPaginated(ctx, 1, 10)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, page, 1)

	beer.Name = "IPA Atualizada"
	err = repo.Update(ctx, beer.ID, beer)
	require.NoError(t, err)

	got, _ = repo.GetByID(ctx, beer.ID)
	require.Equal(t, "IPA Atualizada", got.Name)

	err = repo.Delete(ctx, beer.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, beer.ID)
	require.Error(t, err)
}

func TestIntegration_BeerRepository_SearchAndStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := startPostgresContainer(t)
	defer db.Close()

	err := applyMigrations(db)
	require.NoError(t, err)

	repo, err := NewPostgresBeerRepository(db)
	require.NoError(t, err)

	beers := []model.Beer{
		{ID: "550e8400-e29b-41d4-a716-446655440001", Name: "IPA", Style: "IPA", CreatedBy: "user-1", CreatedAt: "2024-01-01T00:00:00Z", Comments: []model.Comment{}},
		{ID: "550e8400-e29b-41d4-a716-446655440002", Name: "Stout", Style: "Stout", CreatedBy: "user-2", CreatedAt: "2024-01-01T00:00:00Z", Comments: []model.Comment{}},
		{ID: "550e8400-e29b-41d4-a716-446655440003", Name: "IPA Nova", Style: "IPA", CreatedBy: "user-1", CreatedAt: "2024-01-02T00:00:00Z", Comments: []model.Comment{}},
	}
	for _, b := range beers {
		err = repo.Create(ctx, &b)
		require.NoError(t, err)
	}

	results, total, err := repo.SearchBeers(ctx, model.BeerFilters{Query: "IPA", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, results, 2)

	results, total, err = repo.SearchBeers(ctx, model.BeerFilters{Style: "Stout", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, results, 1)

	adminStats, err := repo.GetAdminStats(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, adminStats.TotalBeers)
	require.Len(t, adminStats.TopStyles, 2)

	userStats, err := repo.GetUserStats(ctx, "user-1")
	require.NoError(t, err)
	require.Equal(t, 2, userStats.BeersAdded)
}

func startPostgresContainer(t *testing.T) *sql.DB {
	t.Helper()
	user := os.Getenv("USER")
	if user == "" {
		user = "postgres"
	}
	dsn := fmt.Sprintf("postgres://%s@localhost:5432/beer_test?sslmode=disable", user)
	if envDSN := os.Getenv("TEST_DATABASE_URL"); envDSN != "" {
		dsn = envDSN
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	return db
}

func applyMigrations(db *sql.DB) error {
	migrationDir := "../../../internal/app/migrations"
	order := []string{
		"000_reset.sql",
		"create_users_table.sql",
		"add_role_to_users.sql",
		"add_oauth_provider_to_users.sql",
		"create_beers_table.sql",
		"add_created_by_to_beers.sql",
		"add_created_at_to_beers.sql",
		"add_comments_jsonb.sql",
		"create_indexes.sql",
		"use_uuid_pk.sql",
		"add_updated_at.sql",
		"extend_media.sql",
		"drop_legacy_comments_table.sql",
		"create_push_subscriptions.sql",
		"add_beer_reports.sql",
		"add_beer_deletion_requests.sql",
		"add_moderation_rls.sql",
	}
	for _, name := range order {
		data, err := os.ReadFile(migrationDir + "/" + name)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}


