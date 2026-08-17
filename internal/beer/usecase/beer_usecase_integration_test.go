//go:build integration

package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/repository"
	usermodel "beer-review-app/internal/user/model"
	appErrors "beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/moderation"
	"beer-review-app/pkg/realtime"

	"github.com/stretchr/testify/require"
)

func TestIntegration_BeerUsecase_RBAC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := startPostgresContainer(t)
	defer db.Close()

	err := applyMigrations(db)
	require.NoError(t, err)

	beerRepo, err := repository.NewPostgresBeerRepository(db)
	require.NoError(t, err)

	hub := realtime.NewHub(64)
	uc := NewBeerUsecase(beerRepo, hub, moderation.NewNoopModerator())

	ownerCtx := middleware.WithUserID(ctx, "user-1", "")
	adminCtx := middleware.WithUserID(ctx, "admin-1", usermodel.RoleAdmin)
	intruderCtx := middleware.WithUserID(ctx, "intruder-2", "")

	beer := model.Beer{
		Name: "RBAC Beer",
		Style: "IPA",
		Taste: model.FlavorBitter,
		Aroma: model.AromaCitrus,
		Color: model.ColorAmber,
		Body: model.BodyMedium,
		Carbonation: model.CarbonationMedium,
		Finish: model.FinishDry,
		CreatedBy: "user-1",
		CreatedAt: "2024-01-01T00:00:00Z",
		Comments: []model.Comment{},
	}

	err = uc.Create(ownerCtx, &beer)
	require.NoError(t, err)
	require.NotEmpty(t, beer.ID)

	t.Run("owner can update", func(t *testing.T) {
		beer.Name = "RBAC Beer Updated"
		err := uc.Update(ownerCtx, beer.ID, beer)
		require.NoError(t, err)
	})

	t.Run("owner can delete", func(t *testing.T) {
		err := uc.Delete(ownerCtx, beer.ID)
		require.NoError(t, err)
	})

	beer2 := model.Beer{
		Name: "RBAC Beer 2",
		Style: "Stout",
		Taste: model.FlavorBitter,
		Aroma: model.AromaMalty,
		Color: model.ColorDark,
		Body: model.BodyFull,
		Carbonation: model.CarbonationLow,
		Finish: model.FinishDry,
		CreatedBy: "user-1",
		CreatedAt: "2024-01-01T00:00:00Z",
		Comments: []model.Comment{},
	}
	err = uc.Create(ownerCtx, &beer2)
	require.NoError(t, err)

	t.Run("intruder cannot update", func(t *testing.T) {
		beer2.Name = "Hacked"
		err := uc.Update(intruderCtx, beer2.ID, beer2)
		require.Error(t, err)
		var appErr *appErrors.AppError
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, 403, appErr.Code)
	})

	t.Run("intruder cannot delete", func(t *testing.T) {
		err := uc.Delete(intruderCtx, beer2.ID)
		require.Error(t, err)
		var appErr *appErrors.AppError
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, 403, appErr.Code)
	})

	t.Run("admin can update any beer", func(t *testing.T) {
		beer2.Name = "Admin Updated"
		err := uc.Update(adminCtx, beer2.ID, beer2)
		require.NoError(t, err)
	})

	t.Run("admin can delete any beer", func(t *testing.T) {
		err := uc.Delete(adminCtx, beer2.ID)
		require.NoError(t, err)
	})
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
