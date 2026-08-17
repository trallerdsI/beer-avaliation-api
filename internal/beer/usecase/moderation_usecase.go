package usecase

import (
	"context"
	"net/http"
	"time"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/repository"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/uuid"
)

type ModerationUsecase interface {
	ReportBeer(ctx context.Context, beerID, userID string, input model.BeerReportInput) error
	GetReportsByBeerID(ctx context.Context, beerID string, limit, offset int) ([]model.BeerReport, int, error)
	GetReports(ctx context.Context, filter model.ReportFilter) ([]model.BeerReport, int, error)
	ResolveReport(ctx context.Context, reportID, status, adminID string) error
	RequestDeletion(ctx context.Context, beerID, userID string, input model.BeerDeletionRequestInput) error
	GetDeletionRequests(ctx context.Context, filter model.DeletionRequestFilter) ([]model.BeerDeletionRequest, int, error)
	ResolveDeletionRequest(ctx context.Context, reqID, status, adminID string) error
}

type moderationUsecase struct {
	moderationRepo repository.ModerationRepository
	beerRepo       repository.BeerRepository
}

func NewModerationUsecase(moderationRepo repository.ModerationRepository, beerRepo repository.BeerRepository) ModerationUsecase {
	return &moderationUsecase{
		moderationRepo: moderationRepo,
		beerRepo:       beerRepo,
	}
}

func (u *moderationUsecase) ReportBeer(ctx context.Context, beerID, userID string, input model.BeerReportInput) error {
	report := &model.BeerReport{
		ID:          uuid.MustNewV7(),
		BeerID:      beerID,
		UserID:      userID,
		Reason:      input.Reason,
		Description: input.Description,
		Status:      model.ReportStatusOpen,
		CreatedAt:   time.Now().UTC(),
	}
	return u.moderationRepo.CreateReport(ctx, report)
}

func (u *moderationUsecase) GetReportsByBeerID(ctx context.Context, beerID string, limit, offset int) ([]model.BeerReport, int, error) {
	return u.moderationRepo.GetReportsByBeerID(ctx, beerID, limit, offset)
}

func (u *moderationUsecase) GetReports(ctx context.Context, filter model.ReportFilter) ([]model.BeerReport, int, error) {
	return u.moderationRepo.GetReports(ctx, filter)
}

func (u *moderationUsecase) ResolveReport(ctx context.Context, reportID, status, adminID string) error {
	if !middleware.IsAdmin(ctx) {
		return errors.NewAppError(http.StatusForbidden, "only admins can resolve reports", nil)
	}
	now := time.Now().UTC()
	return u.moderationRepo.ResolveReport(ctx, reportID, status, &adminID, now)
}

func (u *moderationUsecase) RequestDeletion(ctx context.Context, beerID, userID string, input model.BeerDeletionRequestInput) error {
	req := &model.BeerDeletionRequest{
		ID:        uuid.MustNewV7(),
		BeerID:    beerID,
		UserID:    userID,
		Reason:    input.Reason,
		Details:   input.Details,
		Status:    model.DeletionStatusPending,
		CreatedAt: time.Now().UTC(),
	}
	return u.moderationRepo.CreateDeletionRequest(ctx, req)
}

func (u *moderationUsecase) GetDeletionRequests(ctx context.Context, filter model.DeletionRequestFilter) ([]model.BeerDeletionRequest, int, error) {
	return u.moderationRepo.GetDeletionRequests(ctx, filter)
}

func (u *moderationUsecase) ResolveDeletionRequest(ctx context.Context, reqID, status, adminID string) error {
	if !middleware.IsAdmin(ctx) {
		return errors.NewAppError(http.StatusForbidden, "only admins can resolve deletion requests", nil)
	}

	return u.moderationRepo.ExecInTx(ctx, func(ctx context.Context, txRepo repository.ModerationRepository, txBeerRepo repository.BeerRepository) error {
		requests, _, err := txRepo.GetDeletionRequests(ctx, model.DeletionRequestFilter{
			Limit: 1,
			Offset: 0,
		})
		if err != nil {
			return err
		}
		var target *model.BeerDeletionRequest
		for i := range requests {
			if requests[i].ID == reqID {
				target = &requests[i]
				break
			}
		}
		if target == nil {
			return errors.NewAppError(404, "deletion request not found", nil)
		}

		now := time.Now().UTC()
		if err := txRepo.ResolveDeletionRequest(ctx, reqID, status, &adminID, now); err != nil {
			return err
		}
		if status == string(model.DeletionStatusApproved) {
			if err := txBeerRepo.Delete(ctx, target.BeerID); err != nil {
				return errors.NewAppError(500, "failed to delete beer after approving request", err)
			}
		}
		return nil
	})
}
