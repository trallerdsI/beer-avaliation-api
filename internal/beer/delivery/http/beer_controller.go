package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/microcosm-cc/bluemonday"
	"github.com/prometheus/client_golang/prometheus"

	"beer-review-app/pkg/errors"
	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/usecase"
	"beer-review-app/pkg/response"
	"beer-review-app/pkg/validation"

	"go.uber.org/zap"
)

var (
	validate  *validator.Validate
	sanitizer = bluemonday.UGCPolicy()
)

func init() {
	validate = validator.New()
}

type BeerController struct {
	usecase usecase.BeerUsecase
	logger  *zap.Logger // Add logger to the BeerController
}

func NewBeerController(u usecase.BeerUsecase, logger *zap.Logger) *BeerController {
	return &BeerController{usecase: u, logger: logger} // Pass logger during initialization
}

// @Summary Get all beers
// @Description Get a list of all beers with pagination
// @Produce json
// @Success 200 {array} model.Beer
// @Router /beers [get]
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
		c.logger.Error("Failed to retrieve beers", zap.Error(err))
		response.SendError(w, "Failed to retrieve beers", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"beers":    beers,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// @Summary Create a beer
// @Description Create a new beer
// @Accept json
// @Produce json
// @Param beer body model.Beer true "Beer"
// @Success 201 {object} model.Beer
// @Failure 400 {object} map[string]string
// @Router /beers [post]
func (c *BeerController) CreateBeer(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var beer model.Beer
	if err := json.NewDecoder(r.Body).Decode(&beer); err != nil {
		c.logger.Warn("Invalid request body", zap.Error(err))
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		logMetrics(r.Method, r.URL.Path, "400")
		return
	}

	// Initialize fields
	beer.ID = uuid.New().String()
	beer.Comments = make([]model.Comment, 0) // Initialize empty comments array
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)
	

	if err := validate.Struct(beer); err != nil {
		c.logger.Warn("Invalid input", zap.Error(err))
		response.SendError(w, "Invalid input", http.StatusBadRequest)
		logMetrics(r.Method, r.URL.Path, "400")
		return
	}

	// Validate additional beer attributes
	if err := c.validateBeer(beer); err != nil {
		c.logger.Warn(err.Error())
		response.SendError(w, err.Error(), http.StatusBadRequest)
		logMetrics(r.Method, r.URL.Path, "400")
		return
	}

	if err := c.usecase.Create(r.Context(), beer); err != nil {
		c.logger.Error("Failed to create beer", zap.Error(err))
		response.SendError(w, "Failed to create beer", http.StatusInternalServerError)
		logMetrics(r.Method, r.URL.Path, "500")
		return
	}

	prometheus.NewCounter(prometheus.CounterOpts{
		Name: "api_submission_count",
		Help: "Total number of beer submissions",
	}).Inc()

	response.SendResponse(w, http.StatusCreated, beer)

	responseTime := time.Since(start).Seconds()
	prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "api_response_duration_seconds",
		Help:    "Histogram of response durations for API requests",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint"}).
		WithLabelValues(r.Method, r.URL.Path).Observe(responseTime)

	c.logger.Info("Beer created",
		zap.String("name", beer.Name),
		zap.String("style", beer.Style),
		zap.Float64("alcohol", *beer.Alcohol),
	)
}

// @Summary Update a beer
// @Description Update an existing beer by ID
// @Accept json
// @Produce json
// @Param id path string true "Beer ID"
// @Param beer body model.Beer true "Beer"
// @Success 200 {object} model.Beer
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /beers/{id} [put]
func (c *BeerController) UpdateBeer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var beer model.Beer
	if err := json.NewDecoder(r.Body).Decode(&beer); err != nil {
		c.logger.Warn("Invalid request body", zap.Error(err))
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	beer.ID = id
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)

	if err := validate.Struct(beer); err != nil {
		c.logger.Warn("Invalid input", zap.Error(err))
		response.SendError(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if err := c.usecase.Update(r.Context(), id, beer); err != nil {
		c.logger.Error("Failed to update beer", zap.Error(err))
		response.SendError(w, "Failed to update beer", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, beer)
}

// @Summary Delete a beer
// @Description Delete a beer by ID
// @Produce json
// @Param id path string true "Beer ID"
// @Success 204 "No Content"
// @Failure 404 {object} map[string]string
// @Router /beers/{id} [delete]
func (c *BeerController) DeleteBeer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := c.usecase.Delete(r.Context(), id); err != nil {
		c.logger.Error("Failed to delete beer", zap.Error(err))
		response.SendError(w, "Failed to delete beer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Get a beer by ID
// @Description Get a beer by its ID
// @Produce json
// @Param id path string true "Beer ID"
// @Success 200 {object} model.Beer
// @Failure 404 {object} map[string]string
// @Router /beers/{id} [get]
func (c *BeerController) GetBeerByID(w http.ResponseWriter, r *http.Request) {
	beerID := mux.Vars(r)["id"]

	// Fetch beer by ID from the use case
	beer, err := c.usecase.GetByID(r.Context(), beerID)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok && appErr.Code == 404 {
			http.Error(w, appErr.Message, http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to retrieve beer", http.StatusInternalServerError)
		return
	}

	// Return beer if found
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(beer); err != nil {
		http.Error(w, "Failed to encode beer", http.StatusInternalServerError)
	}
}



// @Summary Add a comment to a beer
// @Description Add a comment to a beer by ID
// @Accept json
// @Produce json
// @Param id path string true "Beer ID"
// @Param comment body model.Comment true "Comment"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /beers/{id}/comments [post]
func (c *BeerController) AddComment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Read the body once into a map to check fields
	var rawComment map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawComment); err != nil {
		c.logger.Warn("Invalid request body", zap.Error(err))
		response.SendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check required fields
	if text, ok := rawComment["text"].(string); !ok || text == "" {
		c.logger.Warn("Comment text is required")
		response.SendError(w, "Comment text is required", http.StatusBadRequest)
		return
	}

	if _, exists := rawComment["positive"]; !exists {
		c.logger.Warn("Positive field is required")
		response.SendError(w, "Positive field must be explicitly set to true or false", http.StatusBadRequest)
		return
	}

	// Convert the raw comment to our model
	commentBytes, _ := json.Marshal(rawComment)
	var comment model.Comment
	json.Unmarshal(commentBytes, &comment)

	comment.ID = uuid.New().String()
	comment.Likes = 0

	if err := c.usecase.AddComment(r.Context(), id, comment); err != nil {
		c.logger.Error("Failed to add comment", zap.Error(err))
		response.SendError(w, "Failed to add comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message":   "Comment added successfully",
		"commentId": comment.ID,
	})
}

// @Summary Delete a comment from a beer
// @Description Delete a comment from a beer by ID
// @Accept json
// @Produce json
// @Param id path string true "Beer ID"
// @Param commentId path string true "Comment ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /beers/{id}/comments/{commentId} [delete]
func (c *BeerController) DeleteComment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	beerID := vars["id"]
	commentID := vars["commentId"]

	if err := c.usecase.DeleteComment(r.Context(), beerID, commentID); err != nil {
		c.logger.Error("Failed to delete comment", zap.Error(err))
		response.SendError(w, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{"message": "Comment deleted successfully"})
}

// @Summary Like a comment
// @Description Increment the likes count of a comment
// @Produce json
// @Param id path string true "Beer ID"
// @Param commentId path string true "Comment ID"
// @Param X-Device-ID header string true "Device ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /beers/{id}/comments/{commentId}/like [post]
func (c *BeerController) LikeComment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	beerID := vars["id"]
	commentID := vars["commentId"]

	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		c.logger.Warn("Device ID is required")
		response.SendError(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	if err := c.usecase.LikeComment(r.Context(), beerID, commentID, deviceID); err != nil {
		if err.Error() == "already liked" {
			response.SendError(w, "Comment already liked by this device", http.StatusBadRequest)
			return
		}
		c.logger.Error("Failed to like comment", zap.Error(err))
		response.SendError(w, "Failed to like comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{"message": "Comment liked successfully"})
}

// @Summary Search beers
// @Description Search beers with filters
// @Accept json
// @Produce json
// @Param query query string false "Search term"
// @Param style query string false "Beer style"
// @Param minAlcohol query number false "Minimum alcohol percentage"
// @Param maxAlcohol query number false "Maximum alcohol percentage"
// @Param taste query string false "Taste (Amargo, Doce, Azedo)"
// @Param page query int false "Page number"
// @Param pageSize query int false "Page size"
// @Success 200 {object} []model.Beer
// @Router /beers/search [get]
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
		c.logger.Error("Failed to search beers", zap.Error(err))
		response.SendError(w, "Failed to search beers", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"items":    beers,
		"total":    total,
		"page":     filters.Page,
		"pageSize": filters.PageSize,
	})
}

func getIntParam(query url.Values, key string, defaultValue int) int {
	if val := query.Get(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func logMetrics(method, endpoint, status string) {
	prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "api_request_count",
		Help: "Total number of API requests",
	}, []string{"method", "endpoint", "status"}).
		WithLabelValues(method, endpoint, status).Inc()
}

// validateBeer checks if the beer has valid attributes.
func (c *BeerController) validateBeer(beer model.Beer) error {
	if !validation.IsValidFlavor(beer.Taste) {
		return fmt.Errorf("invalid flavor: %s", beer.Taste)
	}
	if !validation.IsValidAroma(beer.Aroma) {
		return fmt.Errorf("invalid aroma: %s", beer.Aroma)
	}
	if !validation.IsValidColor(beer.Color) {
		return fmt.Errorf("invalid color: %s", beer.Color)
	}
	if !validation.IsValidBody(beer.Body) {
		return fmt.Errorf("invalid body: %s", beer.Body)
	}
	if !validation.IsValidCarbonation(beer.Carbonation) {
		return fmt.Errorf("invalid carbonation: %s", beer.Carbonation)
	}
	if !validation.IsValidFinish(beer.Finish) {
		return fmt.Errorf("invalid finish: %s", beer.Finish)
	}
	return nil
}
