package usecase

import (
	"context"
	"testing"

	"beer-review-app/internal/beer/model"
	usermodel "beer-review-app/internal/user/model"
	appErrors "beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	beerRepo "beer-review-app/internal/beer/repository"
	"beer-review-app/pkg/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestReportBeer(t *testing.T) {
	mockModerationRepo := new(beerRepo.MockModerationRepository)
	mockBeerRepo := new(beerRepo.MockBeerRepository)
	uc := NewModerationUsecase(mockModerationRepo, mockBeerRepo)

	ctx := middleware.WithUserID(context.Background(), "user-1", "")
	mockModerationRepo.On("CreateReport", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		report := args.Get(1).(*model.BeerReport)
		assert.Equal(t, "beer-1", report.BeerID)
		assert.Equal(t, "user-1", report.UserID)
		assert.Equal(t, model.ReportReasonDuplicate, report.Reason)
		assert.Equal(t, model.ReportStatusOpen, report.Status)
		assert.NotEmpty(t, report.ID)
	}).Return(nil)

	input := model.BeerReportInput{Reason: model.ReportReasonDuplicate, Description: "Duplicate beer description"}
	err := uc.ReportBeer(ctx, "beer-1", "user-1", input)

	assert.NoError(t, err)
	mockModerationRepo.AssertExpectations(t)
}

func TestResolveReport_AdminOnly(t *testing.T) {
	mockModerationRepo := new(beerRepo.MockModerationRepository)
	mockBeerRepo := new(beerRepo.MockBeerRepository)
	uc := NewModerationUsecase(mockModerationRepo, mockBeerRepo)

	userCtx := middleware.WithUserID(context.Background(), "user-1", "")
	adminCtx := middleware.WithUserID(context.Background(), "admin-1", usermodel.RoleAdmin)

	err := uc.ResolveReport(userCtx, "report-1", string(model.ReportStatusResolved), "admin-1")
	assert.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, 403, appErr.Code)

	mockModerationRepo.On("ResolveReport", mock.Anything, "report-1", string(model.ReportStatusResolved), mock.Anything, mock.Anything).Return(nil)
	err = uc.ResolveReport(adminCtx, "report-1", string(model.ReportStatusResolved), "admin-1")
	assert.NoError(t, err)
	mockModerationRepo.AssertExpectations(t)
}

func TestRequestDeletion(t *testing.T) {
	mockModerationRepo := new(beerRepo.MockModerationRepository)
	mockBeerRepo := new(beerRepo.MockBeerRepository)
	uc := NewModerationUsecase(mockModerationRepo, mockBeerRepo)

	ctx := middleware.WithUserID(context.Background(), "user-1", "")
	mockModerationRepo.On("CreateDeletionRequest", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(1).(*model.BeerDeletionRequest)
		assert.Equal(t, "beer-1", req.BeerID)
		assert.Equal(t, "user-1", req.UserID)
		assert.Equal(t, model.DeletionReasonDuplicate, req.Reason)
		assert.Equal(t, model.DeletionStatusPending, req.Status)
		assert.NotEmpty(t, req.ID)
	}).Return(nil)

	input := model.BeerDeletionRequestInput{Reason: model.DeletionReasonDuplicate, Details: "Duplicate beer details"}
	err := uc.RequestDeletion(ctx, "beer-1", "user-1", input)

	assert.NoError(t, err)
	mockModerationRepo.AssertExpectations(t)
}

func TestResolveDeletionRequest_AutoDelete(t *testing.T) {
	mockModerationRepo := new(beerRepo.MockModerationRepository)
	mockBeerRepo := new(beerRepo.MockBeerRepository)
	uc := NewModerationUsecase(mockModerationRepo, mockBeerRepo)

	adminCtx := middleware.WithUserID(context.Background(), "admin-1", usermodel.RoleAdmin)
	reqID := uuid.MustNewV7()

	mockModerationRepo.On("ExecInTx", mock.Anything, mock.Anything).Return(nil)

	err := uc.ResolveDeletionRequest(adminCtx, reqID, string(model.DeletionStatusApproved), "admin-1")

	assert.NoError(t, err)
	mockModerationRepo.AssertExpectations(t)
}
