package http

import (
	stdErrors "errors"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/usecase"
	appErrors "beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/response"
)

type ModerationController struct {
	moderationUsecase usecase.ModerationUsecase
	logger            *slog.Logger
}

func NewModerationController(u usecase.ModerationUsecase, logger *slog.Logger) *ModerationController {
	if logger == nil {
		logger = slog.Default()
	}
	return &ModerationController{moderationUsecase: u, logger: logger}
}

func (c *ModerationController) ReportBeer(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")
	userID, _ := middleware.UserIDFromContext(r.Context())

	if r.Body == nil {
		sendAppError(w, r, appErrors.NewAppError(400, "request body is empty", nil))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var input model.BeerReportInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		sendAppError(w, r, appErrors.NewAppError(400, "invalid request body", err))
		return
	}

	if err := validate.Struct(input); err != nil {
		sendAppError(w, r, appErrors.NewAppError(400, validationMessage(err), err))
		return
	}

	if err := c.moderationUsecase.ReportBeer(r.Context(), beerID, userID, input); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) && appErr.Code == 409 {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to report beer", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusCreated, map[string]string{
		"message": "Beer reported successfully",
	})
}

func (c *ModerationController) RequestDeletion(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")
	userID, _ := middleware.UserIDFromContext(r.Context())

	if r.Body == nil {
		sendAppError(w, r, appErrors.NewAppError(400, "request body is empty", nil))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var input model.BeerDeletionRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		sendAppError(w, r, appErrors.NewAppError(400, "invalid request body", err))
		return
	}

	if err := validate.Struct(input); err != nil {
		sendAppError(w, r, appErrors.NewAppError(400, validationMessage(err), err))
		return
	}

	if err := c.moderationUsecase.RequestDeletion(r.Context(), beerID, userID, input); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) && appErr.Code == 409 {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to request deletion", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusCreated, map[string]string{
		"message": "Deletion request submitted successfully",
	})
}

func (c *ModerationController) GetBeerReports(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")
	limit := clampLimit(r.URL.Query().Get("limit"))
	offset := getIntParam(r.URL.Query(), "offset", 0)

	reports, total, err := c.moderationUsecase.GetReportsByBeerID(r.Context(), beerID, limit, offset)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to get beer reports", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"reports": reports,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (c *ModerationController) GetReports(w http.ResponseWriter, r *http.Request) {
	limit := clampLimit(r.URL.Query().Get("limit"))
	offset := getIntParam(r.URL.Query(), "offset", 0)

	filter := model.ReportFilter{
		Limit:  limit,
		Offset: offset,
	}
	if beerID := r.URL.Query().Get("beerId"); beerID != "" {
		filter.BeerID = beerID
	}
	if userID := r.URL.Query().Get("userId"); userID != "" {
		filter.UserID = userID
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = model.ReportStatus(status)
	}

	reports, total, err := c.moderationUsecase.GetReports(r.Context(), filter)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to get reports", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"reports": reports,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (c *ModerationController) ResolveReport(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("id")
	adminID, _ := middleware.UserIDFromContext(r.Context())

	if r.Body == nil {
		sendAppError(w, r, appErrors.NewAppError(400, "request body is empty", nil))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body struct {
		Status string `json:"status" validate:"required,oneof=resolved dismissed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sendAppError(w, r, appErrors.NewAppError(400, "invalid request body", err))
		return
	}

	if err := validate.Struct(body); err != nil {
		sendAppError(w, r, appErrors.NewAppError(400, validationMessage(err), err))
		return
	}

	if err := c.moderationUsecase.ResolveReport(r.Context(), reportID, body.Status, adminID); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to resolve report", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message": "Report resolved successfully",
	})
}

func (c *ModerationController) ResolveDeletionRequest(w http.ResponseWriter, r *http.Request) {
	reqID := r.PathValue("id")
	adminID, _ := middleware.UserIDFromContext(r.Context())

	if r.Body == nil {
		sendAppError(w, r, appErrors.NewAppError(400, "request body is empty", nil))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body struct {
		Status string `json:"status" validate:"required,oneof=approved rejected"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sendAppError(w, r, appErrors.NewAppError(400, "invalid request body", err))
		return
	}

	if err := validate.Struct(body); err != nil {
		sendAppError(w, r, appErrors.NewAppError(400, validationMessage(err), err))
		return
	}

	if err := c.moderationUsecase.ResolveDeletionRequest(r.Context(), reqID, body.Status, adminID); err != nil {
		var appErr *appErrors.AppError
		if stdErrors.As(err, &appErr) {
			sendAppError(w, r, appErr)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to resolve deletion request", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message": "Deletion request resolved successfully",
	})
}

func (c *ModerationController) GetDeletionRequests(w http.ResponseWriter, r *http.Request) {
	limit := clampLimit(r.URL.Query().Get("limit"))
	offset := getIntParam(r.URL.Query(), "offset", 0)

	filter := model.DeletionRequestFilter{
		Limit:  limit,
		Offset: offset,
	}
	if beerID := r.URL.Query().Get("beerId"); beerID != "" {
		filter.BeerID = beerID
	}
	if userID := r.URL.Query().Get("userId"); userID != "" {
		filter.UserID = userID
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = model.DeletionStatus(status)
	}

	requests, total, err := c.moderationUsecase.GetDeletionRequests(r.Context(), filter)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to get deletion requests", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"deletionRequests": requests,
		"total":            total,
		"limit":            limit,
		"offset":           offset,
	})
}

func clampLimit(raw string) int {
	if raw == "" {
		return 20
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 20
	}
	if n > 100 {
		return 100
	}
	return n
}
