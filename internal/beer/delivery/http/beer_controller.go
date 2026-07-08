package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/microcosm-cc/bluemonday"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/usecase"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/response"
)

var (
	validate  *validator.Validate
	sanitizer = bluemonday.UGCPolicy()

	beerSubmissionCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "api_submission_count",
		Help: "Total number of beer submissions",
	})
	responseDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "api_response_duration_seconds",
		Help:    "Histogram of response durations for API requests",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint"})
)

func init() {
	validate = validator.New()
	prometheus.MustRegister(beerSubmissionCounter, responseDurationHistogram)
}

// BeerController handles HTTP requests related to beers.
type BeerController struct {
	usecase usecase.BeerUsecase
	logger  *zap.Logger
}

// makes new controller for beer
func NewBeerController(u usecase.BeerUsecase, logger *zap.Logger) *BeerController {
	return &BeerController{usecase: u, logger: logger}
}

func getIntParam(query url.Values, key string, defaultValue int) int {
	param := query.Get(key)
	if param == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(param)
	if err != nil {
		return defaultValue
	}
	return value
}

func handleError(w http.ResponseWriter, logger *zap.Logger, err error, message string, statusCode int) {
	logger.Error(message, zap.Error(err))
	response.SendError(w, message, statusCode)
	logMetrics("POST", "/beers", strconv.Itoa(statusCode))
}

func logMetrics(method, endpoint, status string) {
	prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "api_request_count",
		Help: "Total number of API requests",
	}, []string{"method", "endpoint", "status"}).
		WithLabelValues(method, endpoint, status).Inc()
}

func (c *BeerController) validateBeer(beer model.Beer) error {
	if beer.Name == "" {
		return fmt.Errorf("beer name cannot be empty")
	}
	if beer.Style == "" {
		return fmt.Errorf("beer style cannot be empty")
	}

	return nil
}

func requestParam(r *http.Request, key string) string {
	if vars := mux.Vars(r); len(vars) > 0 {
		if value, ok := vars[key]; ok && value != "" {
			return value
		}
	}

	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i := 0; i < len(segments)-1; i++ {
		if segments[i] == "beers" && key == "id" {
			return segments[i+1]
		}
		if segments[i] == "comments" && key == "commentId" && i+1 < len(segments) {
			return segments[i+1]
		}
	}

	return ""
}

func (c *BeerController) GetAllBeers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 10
	}

	beers, total, err := c.usecase.GetPaginated(r.Context(), page, pageSize)
	if err != nil {
		handleError(w, c.logger, err, "Failed to retrieve beers", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"beers":    beers,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (c *BeerController) CreateBeer(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var beer model.Beer
	if r.Body == nil {
		handleError(w, c.logger, fmt.Errorf("request body is empty"), "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&beer); err != nil {
		handleError(w, c.logger, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	beer.ID = uuid.New().String()
	beer.Comments = make([]model.Comment, 0)
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)

	if err := c.validateBeer(beer); err != nil {
		handleError(w, c.logger, err, err.Error(), http.StatusBadRequest)
		return
	}

	if err := c.usecase.Create(r.Context(), beer); err != nil {
		handleError(w, c.logger, err, "Failed to create beer", http.StatusInternalServerError)
		return
	}

	beerSubmissionCounter.Inc()
	response.SendResponse(w, http.StatusCreated, beer)

	responseTime := time.Since(start).Seconds()
	responseDurationHistogram.WithLabelValues(r.Method, r.URL.Path).Observe(responseTime)

	c.logger.Info("Beer created", zap.String("name", beer.Name), zap.String("style", beer.Style))
}

func (c *BeerController) UpdateBeer(w http.ResponseWriter, r *http.Request) {
	id := requestParam(r, "id")

	var beer model.Beer
	if r.Body == nil {
		handleError(w, c.logger, fmt.Errorf("request body is empty"), "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&beer); err != nil {
		handleError(w, c.logger, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	beer.ID = id
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)

	if err := c.validateBeer(beer); err != nil {
		handleError(w, c.logger, err, err.Error(), http.StatusBadRequest)
		return
	}

	if err := c.usecase.Update(r.Context(), id, beer); err != nil {
		handleError(w, c.logger, err, "Failed to update beer", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, beer)
}

func (c *BeerController) DeleteBeer(w http.ResponseWriter, r *http.Request) {
	id := requestParam(r, "id")

	if err := c.usecase.Delete(r.Context(), id); err != nil {
		handleError(w, c.logger, err, "Failed to delete beer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *BeerController) GetBeerByID(w http.ResponseWriter, r *http.Request) {
	beerID := requestParam(r, "id")

	beer, err := c.usecase.GetByID(r.Context(), beerID)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok && (appErr.Code == 404 || strings.Contains(strings.ToLower(appErr.Message), "not found")) {
			http.Error(w, appErr.Message, http.StatusNotFound)
			return
		}
		handleError(w, c.logger, err, "Failed to retrieve beer", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(beer); err != nil {
		handleError(w, c.logger, err, "Failed to encode beer", http.StatusInternalServerError)
	}
}

func (c *BeerController) AddComment(w http.ResponseWriter, r *http.Request) {
	id := requestParam(r, "id")

	var comment model.Comment
	if r.Body == nil {
		handleError(w, c.logger, fmt.Errorf("request body is empty"), "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		handleError(w, c.logger, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(comment); err != nil {
		handleError(w, c.logger, err, "Invalid input", http.StatusBadRequest)
		return
	}

	comment.ID = uuid.New().String()
	comment.Likes = 0

	if err := c.usecase.AddComment(r.Context(), id, comment); err != nil {
		handleError(w, c.logger, err, "Failed to add comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message":   "Comment added successfully",
		"commentId": comment.ID,
	})
}

func (c *BeerController) DeleteComment(w http.ResponseWriter, r *http.Request) {
	beerID := requestParam(r, "id")
	commentID := requestParam(r, "commentId")

	if err := c.usecase.DeleteComment(r.Context(), beerID, commentID); err != nil {
		if appErr, ok := err.(*errors.AppError); ok && (appErr.Code == http.StatusNotFound || strings.Contains(strings.ToLower(appErr.Message), "not found")) {
			http.Error(w, appErr.Message, http.StatusNotFound)
			return
		}
		handleError(w, c.logger, err, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{"message": "Comment deleted successfully"})
}

func (c *BeerController) LikeComment(w http.ResponseWriter, r *http.Request) {
	beerID := requestParam(r, "id")
	commentID := requestParam(r, "commentId")

	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		handleError(w, c.logger, fmt.Errorf("device ID is required"), "Device ID is required", http.StatusBadRequest)
		return
	}

	if err := c.usecase.LikeComment(r.Context(), beerID, commentID, deviceID); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			if appErr.Code == http.StatusBadRequest && strings.Contains(strings.ToLower(appErr.Message), "already liked") {
				handleError(w, c.logger, err, "Comment already liked by this device", http.StatusBadRequest)
				return
			}
			if appErr.Code == http.StatusNotFound || strings.Contains(strings.ToLower(appErr.Message), "not found") {
				http.Error(w, appErr.Message, http.StatusNotFound)
				return
			}
		}
		handleError(w, c.logger, err, "Failed to like comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{"message": "Comment liked successfully"})
}

func (c *BeerController) SearchBeers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filters := model.BeerFilters{
		Query:    query.Get("query"),
		Style:    query.Get("style"),
		Taste:    query.Get("taste"),
		Page:     getIntParam(query, "page", 1),
		PageSize: getIntParam(query, "pageSize", 10),
	}

	if minAlc := query.Get("minAlcohol"); minAlc != "" {
		if val, err := strconv.ParseFloat(minAlc, 64); err == nil {
			filters.MinAlcohol = &val
		}
	}

	if maxAlc := query.Get("maxAlcohol"); maxAlc != "" {
		if val, err := strconv.ParseFloat(maxAlc, 64); err == nil {
			filters.MaxAlcohol = &val
		}
	}

	beers, total, err := c.usecase.SearchBeers(r.Context(), filters)
	if err != nil {
		handleError(w, c.logger, err, "Failed to search beers", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"beers":    beers,
		"total":    total,
		"page":     filters.Page,
		"pageSize": filters.PageSize,
	})
}
