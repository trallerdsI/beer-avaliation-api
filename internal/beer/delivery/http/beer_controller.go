package http

import (
	"context"
	"encoding/json"
	stdErrors "errors" // Renomeado para evitar conflito com o pacote de erros customizado
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/usecase"
	appErrors "beer-review-app/pkg/errors" // Alias explícito para evitar confusão
	"beer-review-app/pkg/response"
)

// validate é um validador de structs de stack (zero-allocation por request no
// caminho crítico: o ponteiro vive no package scope, sem new() por requisição).
var (
	validate  = validator.New()
	sanitizer = bluemonday.UGCPolicy()
)

// BeerController handles HTTP requests related to beers.
type BeerController struct {
	usecase usecase.BeerUsecase
	logger  *slog.Logger
}

// NewBeerController makes a new controller for beer
func NewBeerController(u usecase.BeerUsecase, logger *slog.Logger) *BeerController {
	if logger == nil {
		logger = slog.Default()
	}
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

func handleError(w http.ResponseWriter, ctx context.Context, logger *slog.Logger, err error, message string, statusCode int) {
	logger.ErrorContext(ctx, message, "err", err)
	// Expõe a causa raiz (ex: erro de DB) no campo "detail" para diagnóstico,
	// preservando a mensagem genérica no cliente.
	response.SendError(w, message, statusCode, rootCause(err))
}

// rootCause devolve a mensagem da causa mais interna do erro, ou "" se ausente.
func rootCause(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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

func (c *BeerController) GetAllBeers(w http.ResponseWriter, r *http.Request) {
	// REUTILIZADO: Uso consistente do helper 'getIntParam'
	query := r.URL.Query()
	page := getIntParam(query, "page", 1)
	pageSize := getIntParam(query, "pageSize", 10)

	beers, total, err := c.usecase.GetPaginated(r.Context(), page, pageSize)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to retrieve beers", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"beers":    response.SelectFields(beers, query.Get("fields")),
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (c *BeerController) CreateBeer(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		handleError(w, r.Context(), c.logger, fmt.Errorf("request body is empty"), "Invalid request body", http.StatusBadRequest)
		return
	}

	// SEGURANÇA: Limita o tamanho do JSON para 1MB evitando Denial of Service (DoS) por memória excedida
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var beer model.Beer
	if err := json.NewDecoder(r.Body).Decode(&beer); err != nil {
		handleError(w, r.Context(), c.logger, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	beer.ID = uuid.New().String()
	beer.Comments = make([]model.Comment, 0)
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)

	if err := c.validateBeer(beer); err != nil {
		handleError(w, r.Context(), c.logger, err, err.Error(), http.StatusBadRequest)
		return
	}

	if err := c.usecase.Create(r.Context(), beer); err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to create beer", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusCreated, beer)

	c.logger.Info("Beer created", slog.String("name", beer.Name), slog.String("style", beer.Style))
}

func (c *BeerController) UpdateBeer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if r.Body == nil {
		handleError(w, r.Context(), c.logger, fmt.Errorf("request body is empty"), "Invalid request body", http.StatusBadRequest)
		return
	}

	// SEGURANÇA: Limitação de carga útil
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var beer model.Beer
	if err := json.NewDecoder(r.Body).Decode(&beer); err != nil {
		handleError(w, r.Context(), c.logger, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	beer.ID = id
	beer.Name = sanitizer.Sanitize(beer.Name)
	beer.Description = sanitizer.Sanitize(beer.Description)

	if err := c.validateBeer(beer); err != nil {
		handleError(w, r.Context(), c.logger, err, err.Error(), http.StatusBadRequest)
		return
	}

	if err := c.usecase.Update(r.Context(), id, beer); err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to update beer", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, beer)
}

func (c *BeerController) DeleteBeer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := c.usecase.Delete(r.Context(), id); err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to delete beer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *BeerController) GetBeerByID(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")

	beer, err := c.usecase.GetByID(r.Context(), beerID)
	if err != nil {
		var appErr *appErrors.AppError
		// CORRIGIDO: Uso moderno de errors.As para segurança e compatibilidade com wrapping
		if stdErrors.As(err, &appErr) && (appErr.Code == http.StatusNotFound || strings.Contains(strings.ToLower(appErr.Message), "not found")) {
			http.Error(w, appErr.Message, http.StatusNotFound)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to retrieve beer", http.StatusInternalServerError)
		return
	}

	// PADRONIZADO: Uso do helper global de respostas JSON em vez de encode cru manual
	response.SendResponse(w, http.StatusOK, beer)
}

func (c *BeerController) AddComment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if r.Body == nil {
		handleError(w, r.Context(), c.logger, fmt.Errorf("request body is empty"), "Invalid request body", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var comment model.Comment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		handleError(w, r.Context(), c.logger, err, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := validate.Struct(comment); err != nil {
		handleError(w, r.Context(), c.logger, err, "Invalid input", http.StatusBadRequest)
		return
	}

	comment.ID = uuid.New().String()
	comment.Likes = 0

	if err := c.usecase.AddComment(r.Context(), id, comment); err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to add comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{
		"message":   "Comment added successfully",
		"commentId": comment.ID,
	})
}

func (c *BeerController) DeleteComment(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")
	commentID := r.PathValue("commentId")

	if err := c.usecase.DeleteComment(r.Context(), beerID, commentID); err != nil {
		var appErr *appErrors.AppError
		// CORRIGIDO: Uso de errors.As
		if stdErrors.As(err, &appErr) && (appErr.Code == http.StatusNotFound || strings.Contains(strings.ToLower(appErr.Message), "not found")) {
			http.Error(w, appErr.Message, http.StatusNotFound)
			return
		}
		handleError(w, r.Context(), c.logger, err, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]string{"message": "Comment deleted successfully"})
}

func (c *BeerController) LikeComment(w http.ResponseWriter, r *http.Request) {
	beerID := r.PathValue("id")
	commentID := r.PathValue("commentId")

	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		handleError(w, r.Context(), c.logger, fmt.Errorf("device ID is required"), "Device ID is required", http.StatusBadRequest)
		return
	}

	if err := c.usecase.LikeComment(r.Context(), beerID, commentID, deviceID); err != nil {
		var appErr *appErrors.AppError
		// CORRIGIDO: Uso de errors.As
		if stdErrors.As(err, &appErr) {
			if appErr.Code == http.StatusBadRequest && strings.Contains(strings.ToLower(appErr.Message), "already liked") {
				handleError(w, r.Context(), c.logger, err, "Comment already liked by this device", http.StatusBadRequest)
				return
			}
			if appErr.Code == http.StatusNotFound || strings.Contains(strings.ToLower(appErr.Message), "not found") {
				http.Error(w, appErr.Message, http.StatusNotFound)
				return
			}
		}
		handleError(w, r.Context(), c.logger, err, "Failed to like comment", http.StatusInternalServerError)
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
			filters.MinAlcohol = new(float64(val))
		}
	}

	if maxAlc := query.Get("maxAlcohol"); maxAlc != "" {
		if val, err := strconv.ParseFloat(maxAlc, 64); err == nil {
			filters.MaxAlcohol = new(float64(val))
		}
	}

	beers, total, err := c.usecase.SearchBeers(r.Context(), filters)
	if err != nil {
		handleError(w, r.Context(), c.logger, err, "Failed to search beers", http.StatusInternalServerError)
		return
	}

	response.SendResponse(w, http.StatusOK, map[string]interface{}{
		"beers":    beers,
		"total":    total,
		"page":     filters.Page,
		"pageSize": filters.PageSize,
	})
}
