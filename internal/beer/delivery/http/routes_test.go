package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// GET /api/v1/beers  (GetAllBeers)
// ---------------------------------------------------------------------------

func TestGetAllBeers_Success(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	mockBeerUsecase.On("GetPaginated", mock.Anything, 1, 10).
		Return([]model.Beer{{ID: "1", Name: "Beer1"}, {ID: "2", Name: "Beer2"}}, 2, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers?page=1&pageSize=10", nil)
	rr := httptest.NewRecorder()
	controller.GetAllBeers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Beers []model.Beer `json:"beers"`
		Total int          `json:"total"`
		Page  int          `json:"page"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	assert.Len(t, body.Beers, 2)
	assert.Equal(t, 2, body.Total)
	assert.Equal(t, 1, body.Page)
}

func TestGetAllBeers_UsecaseError(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	mockBeerUsecase.On("GetPaginated", mock.Anything, 1, 10).
		Return(nil, 0, errors.NewAppError(500, "db down", nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers", nil)
	rr := httptest.NewRecorder()
	controller.GetAllBeers(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "db down")
}

func TestGetAllBeers_DefaultPagination(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	// Sem query params => defaults page=1, pageSize=10.
	mockBeerUsecase.On("GetPaginated", mock.Anything, 1, 10).
		Return([]model.Beer{}, 0, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers", nil)
	rr := httptest.NewRecorder()
	controller.GetAllBeers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockBeerUsecase.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// GET /api/v1/feed  (GetHomeFeed / BFF)
// ---------------------------------------------------------------------------

func TestGetHomeFeed_Success(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	mockBeerUsecase.On("GetPaginated", mock.Anything, 1, 20).
		Return([]model.Beer{{ID: "1", Name: "B", Comments: []model.Comment{{ID: "c1"}}}}, 1, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feed", nil)
	rr := httptest.NewRecorder()
	controller.GetHomeFeed(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Featured []struct {
			CommentCount int `json:"commentCount"`
		} `json:"featured"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	assert.Len(t, body.Featured, 1)
	assert.Equal(t, 1, body.Featured[0].CommentCount)
}

func TestGetHomeFeed_UsecaseError(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	mockBeerUsecase.On("GetPaginated", mock.Anything, 1, 20).
		Return(nil, 0, errors.NewAppError(500, "feed unavailable", nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feed", nil)
	rr := httptest.NewRecorder()
	controller.GetHomeFeed(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// GET /api/v1/beers/search  (SearchBeers)
// ---------------------------------------------------------------------------

func TestSearchBeers_Success(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	mockBeerUsecase.On("SearchBeers", mock.Anything, mock.Anything).
		Return([]model.Beer{{ID: "1", Name: "IPA"}}, 1, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers/search?query=ipa&style=IPA&minAlcohol=4.5&maxAlcohol=8", nil)
	rr := httptest.NewRecorder()
	controller.SearchBeers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Beers []model.Beer `json:"beers"`
		Total int          `json:"total"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	assert.Len(t, body.Beers, 1)
	assert.Equal(t, 1, body.Total)
}

func TestSearchBeers_UsecaseError(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	mockBeerUsecase.On("SearchBeers", mock.Anything, mock.Anything).
		Return(nil, 0, errors.NewAppError(500, "search failed", nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers/search?query=x", nil)
	rr := httptest.NewRecorder()
	controller.SearchBeers(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// DELETE /api/v1/beers/{id}  (DeleteBeer)
// ---------------------------------------------------------------------------

func TestDeleteBeer_Success(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	mockBeerUsecase.On("Delete", mock.Anything, "1").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/beers/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	controller.DeleteBeer(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestDeleteBeer_UsecaseError(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	mockBeerUsecase.On("Delete", mock.Anything, "1").
		Return(errors.NewAppError(404, "beer not found", nil))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/beers/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	controller.DeleteBeer(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// PUT /api/v1/beers/{id}  (UpdateBeer) — corpo inválido
// ---------------------------------------------------------------------------

func TestUpdateBeer_InvalidBody(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/beers/1", bytes.NewReader([]byte("not-json{")))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	controller.UpdateBeer(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---------------------------------------------------------------------------
// POST /api/v1/beers  (CreateBeer) — validação de campos obrigatórios
// ---------------------------------------------------------------------------

func TestCreateBeer_MissingName(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	// Nome vazio => validateBeer falha (400) antes de chamar o usecase.
	body, _ := json.Marshal(model.Beer{Style: "IPA"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	controller.CreateBeer(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	// O usecase NÃO deve ser chamado.
	mockBeerUsecase.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

