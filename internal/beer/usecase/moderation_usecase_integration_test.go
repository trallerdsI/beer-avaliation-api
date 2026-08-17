//go:build integration

package usecase

import (
	"context"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/repository"
	usermodel "beer-review-app/internal/user/model"
	appErrors "beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/uuid"

	"github.com/stretchr/testify/require"
)

func TestIntegration_ModerationUsecase_Reports(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := startPostgresContainer(t)
	defer db.Close()

	err := applyMigrations(db)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO beerUsers (id, username, email, password, created)
		VALUES ('00000000-0000-0000-0000-000000000001', 'user-1', 'user1@test.com', 'hash', now())
		ON CONFLICT (id) DO NOTHING
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO beerUsers (id, username, email, password, created)
		VALUES ('00000000-0000-0000-0000-000000000002', 'admin-1', 'admin1@test.com', 'hash', now())
		ON CONFLICT (id) DO NOTHING
	`)
	require.NoError(t, err)

	beerRepo, err := repository.NewPostgresBeerRepository(db)
	require.NoError(t, err)

	moderationRepo, err := repository.NewPostgresModerationRepository(db)
	require.NoError(t, err)

	uc := NewModerationUsecase(moderationRepo, beerRepo)

	beer := model.Beer{
		ID:          uuid.MustNewV7(),
		Name:        "Moderation Test Beer",
		Style:       "IPA",
		Taste:       model.FlavorBitter,
		Aroma:       model.AromaCitrus,
		Color:       model.ColorAmber,
		Body:        model.BodyMedium,
		Carbonation: model.CarbonationMedium,
		Finish:      model.FinishDry,
		CreatedBy:   "00000000-0000-0000-0000-000000000001",
		CreatedAt:   "2024-01-01T00:00:00Z",
		Comments:    []model.Comment{},
	}
	err = beerRepo.Create(ctx, &beer)
	require.NoError(t, err)

	userCtx := middleware.WithUserID(ctx, "00000000-0000-0000-0000-000000000001", "")
	adminCtx := middleware.WithUserID(ctx, "00000000-0000-0000-0000-000000000002", usermodel.RoleAdmin)

	err = uc.ReportBeer(userCtx, beer.ID, "00000000-0000-0000-0000-000000000001", model.BeerReportInput{
		Reason:      model.ReportReasonDuplicate,
		Description: "Duplicate beer for integration testing",
	})
	require.NoError(t, err)

	reports, total, err := uc.GetReportsByBeerID(userCtx, beer.ID, 10, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, reports, 1)
	require.Equal(t, model.ReportReasonDuplicate, reports[0].Reason)
	require.Equal(t, model.ReportStatusOpen, reports[0].Status)

	err = uc.ResolveReport(adminCtx, reports[0].ID, string(model.ReportStatusResolved), "00000000-0000-0000-0000-000000000002")
	require.NoError(t, err)

	reports, total, err = uc.GetReportsByBeerID(userCtx, beer.ID, 10, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, reports, 1)
	require.Equal(t, model.ReportStatusResolved, reports[0].Status)
	require.NotNil(t, reports[0].ResolvedBy)
	require.NotNil(t, reports[0].ResolvedAt)
}

func TestIntegration_ModerationUsecase_DeletionRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := startPostgresContainer(t)
	defer db.Close()

	err := applyMigrations(db)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO beerUsers (id, username, email, password, created)
		VALUES ('00000000-0000-0000-0000-000000000001', 'user-1', 'user1@test.com', 'hash', now())
		ON CONFLICT (id) DO NOTHING
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO beerUsers (id, username, email, password, created)
		VALUES ('00000000-0000-0000-0000-000000000002', 'admin-1', 'admin1@test.com', 'hash', now())
		ON CONFLICT (id) DO NOTHING
	`)
	require.NoError(t, err)

	beerRepo, err := repository.NewPostgresBeerRepository(db)
	require.NoError(t, err)

	moderationRepo, err := repository.NewPostgresModerationRepository(db)
	require.NoError(t, err)

	uc := NewModerationUsecase(moderationRepo, beerRepo)

	beer := model.Beer{
		ID:          uuid.MustNewV7(),
		Name:        "Deletion Test Beer",
		Style:       "Stout",
		Taste:       model.FlavorBitter,
		Aroma:       model.AromaMalty,
		Color:       model.ColorDark,
		Body:        model.BodyFull,
		Carbonation: model.CarbonationLow,
		Finish:      model.FinishDry,
		CreatedBy:   "00000000-0000-0000-0000-000000000001",
		CreatedAt:   "2024-01-01T00:00:00Z",
		Comments:    []model.Comment{},
	}
	err = beerRepo.Create(ctx, &beer)
	require.NoError(t, err)

	userCtx := middleware.WithUserID(ctx, "00000000-0000-0000-0000-000000000001", "")
	adminCtx := middleware.WithUserID(ctx, "00000000-0000-0000-0000-000000000002", usermodel.RoleAdmin)

	err = uc.RequestDeletion(userCtx, beer.ID, "00000000-0000-0000-0000-000000000001", model.BeerDeletionRequestInput{
		Reason:  model.DeletionReasonDuplicate,
		Details: "Duplicate beer for integration testing",
	})
	require.NoError(t, err)

	requests, total, err := uc.GetDeletionRequests(userCtx, model.DeletionRequestFilter{})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, requests, 1)
	require.Equal(t, model.DeletionReasonDuplicate, requests[0].Reason)
	require.Equal(t, model.DeletionStatusPending, requests[0].Status)

	err = uc.ResolveDeletionRequest(adminCtx, requests[0].ID, string(model.DeletionStatusApproved), "00000000-0000-0000-0000-000000000002")
	require.NoError(t, err)

	_, err = beerRepo.GetByID(ctx, beer.ID)
	require.Error(t, err)
	var appErr *appErrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 404, appErr.Code)
}
