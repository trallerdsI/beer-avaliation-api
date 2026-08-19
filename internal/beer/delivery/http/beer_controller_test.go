package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/usecase"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"

	"github.com/stretchr/testify/mock"
)

// Create mock dependencies
var (
	mockLogger      = slog.Default()
	mockBeerUsecase = new(MockBeerUsecase)
)

// MockBeerUsecase is a mock implementation of BeerUsecase
type MockBeerUsecase struct {
	usecase.BeerUsecase
	LikeCommentFunc func(ctx context.Context, beerID, commentID, userID, deviceID string) error
	mock.Mock
}

func (m *MockBeerUsecase) GetAll(ctx context.Context) ([]model.Beer, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Beer), args.Error(1)
}

func (m *MockBeerUsecase) Create(ctx context.Context, beer *model.Beer) error {
	args := m.Called(ctx, beer)
	return args.Error(0)
}

func (m *MockBeerUsecase) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]model.Beer), args.Int(1), args.Error(2)
}

func (m *MockBeerUsecase) Update(ctx context.Context, id string, beer model.Beer) error {
	args := m.Called(ctx, id, beer)
	return args.Error(0)
}

func (m *MockBeerUsecase) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockBeerUsecase) AddComment(ctx context.Context, id string, comment model.Comment) error {
	args := m.Called(ctx, id, comment)
	return args.Error(0)
}

func (m *MockBeerUsecase) DeleteComment(ctx context.Context, beerID, commentID string) error {
	args := m.Called(ctx, beerID, commentID)
	return args.Error(0)
}

func (m *MockBeerUsecase) LikeComment(ctx context.Context, beerID, commentID, userID, deviceID string) error {
	if m.LikeCommentFunc != nil {
		return m.LikeCommentFunc(ctx, beerID, commentID, userID, deviceID)
	}
	args := m.Called(ctx, beerID, commentID, userID, deviceID)
	return args.Error(0)
}

func (m *MockBeerUsecase) AddMedia(ctx context.Context, id string, item model.MediaItem) ([]model.MediaItem, error) {
	args := m.Called(ctx, id, item)
	return args.Get(0).([]model.MediaItem), args.Error(1)
}

func (m *MockBeerUsecase) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, bool, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]model.Beer), args.Int(1), args.Bool(2), args.Error(3)
}

func (m *MockBeerUsecase) GetByID(ctx context.Context, id string) (model.Beer, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.Beer), args.Error(1)
}

// TestGetBeer tests the GET /beers/{id} endpoint for retrieving a beer by its ID.
func TestGetBeer(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	// Set up a helper function to send a request and check the status code
	makeRequest := func(url string) *httptest.ResponseRecorder {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.SetPathValue("id", strings.TrimPrefix(url, "/beers/"))
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(controller.GetBeerByID)
		handler.ServeHTTP(rr, req)
		return rr
	}

	// Test for a valid beer ID
	mockBeerUsecase.On("GetByID", context.Background(), "1").Return(model.Beer{ID: "1", Name: "Test Beer"}, nil)

	// Request for beer ID 1
	rr := makeRequest("/beers/1")
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Additional test case for non-existent beer ID
	mockBeerUsecase.On("GetByID", context.Background(), "99").Return(model.Beer{}, errors.NewAppError(http.StatusNotFound, "not found", nil))

	// Request for non-existent beer ID 99
	rr = makeRequest("/beers/99")
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for non-existent beer: got %v want %v", status, http.StatusNotFound)
	}
	// O 404 deve vir em envelope RFC 7807 consistente (não texto plano) — regressão
	// do Bug 1 (GetBeerByID usava http.Error).
	if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("404 deve ser application/problem+json, got Content-Type %q", ct)
	}
	var notFoundBody struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &notFoundBody); err != nil {
		t.Errorf("404 body should be valid JSON, got %q: %v", rr.Body.String(), err)
	}
	if notFoundBody.Status != http.StatusNotFound {
		t.Errorf("404 status mismatch: got %d", notFoundBody.Status)
	}
	if notFoundBody.Code != "not_found" {
		t.Errorf("404 code mismatch: got %q", notFoundBody.Code)
	}
	if notFoundBody.Detail != "not found" {
		t.Errorf("404 detail mismatch: got %q", notFoundBody.Detail)
	}
}

// TestCreateBeer tests the POST /beers endpoint for creating a new beer.
func TestCreateBeer(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	// Test for successful beer creation
	mockBeerUsecase.On("Create", context.Background(), mock.Anything).Return(nil).Once()
	var abvTest = 5.5
	beer := model.Beer{Name: "Test Beer", Style: "IPA", Description: "desc", ImageUrl: "https://x.com/a.jpg", Alcohol: &abvTest, Taste: "Doce", Aroma: "Floral", Color: "Clara", Body: "Leve", Carbonation: "Baixa", Finish: "Seco"}
	body, _ := json.Marshal(beer)
	req, err := http.NewRequest("POST", "/beers", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(controller.CreateBeer)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	// Additional test case for invalid beer data
	req, err = http.NewRequest("POST", "/beers", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid data: got %v want %v", status, http.StatusBadRequest)
	}
}

// TestUpdateBeer tests the PUT /beers/{id} endpoint for updating an existing beer.
func TestUpdateBeer(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	// Test for successful update
	mockBeerUsecase.On("Update", context.Background(), "1", mock.Anything).Return(nil).Once()
	var abvUpd = 5.5
	beer := model.Beer{Name: "Updated Beer", Style: "IPA", Description: "desc", ImageUrl: "https://x.com/a.jpg", Alcohol: &abvUpd, Taste: "Doce", Aroma: "Floral", Color: "Clara", Body: "Leve", Carbonation: "Baixa", Finish: "Seco"}
	body, _ := json.Marshal(beer)
	req, err := http.NewRequest("PUT", "/beers/1", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(controller.UpdateBeer)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code for update: got %v want %v", status, http.StatusOK)
	}
}

// TestDeleteBeer tests the DELETE /beers/{id} endpoint for deleting a beer.
func TestDeleteBeer(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	// Test for successful deletion
	mockBeerUsecase.On("Delete", context.Background(), "1").Return(nil).Once()
	req, err := http.NewRequest("DELETE", "/beers/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(controller.DeleteBeer)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code for deletion: got %v want %v", status, http.StatusNoContent)
	}
}

// TestAddComment tests the POST /beers/{id}/comments endpoint for adding a comment to a beer.
func TestAddComment(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	// Test case 1: Valid comment
	t.Run("Valid Comment", func(t *testing.T) {
		mockBeerUsecase.On("AddComment", context.Background(), "1", mock.Anything).Return(nil).Once()
		comment := model.Comment{Text: "Great beer!", Rating: 5}
		body, _ := json.Marshal(comment)
		req := httptest.NewRequest("POST", "/beers/1/comments", bytes.NewBuffer(body))
		req.SetPathValue("id", "1")
		rr := httptest.NewRecorder()

		controller.AddComment(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// Test case 2: Missing text
	t.Run("Missing Text", func(t *testing.T) {
		comment := model.Comment{Rating: 5}
		body, _ := json.Marshal(comment)
		req := httptest.NewRequest("POST", "/beers/1/comments", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		controller.AddComment(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	// Test case 4: Inappropriate content blocked by moderation
	t.Run("Inappropriate Content Blocked", func(t *testing.T) {
		mockBeerUsecase.On("AddComment", context.Background(), "1", mock.Anything).
			Return(errors.NewAppErrorWithCode(http.StatusUnprocessableEntity, "O comentário viola as diretrizes de conteúdo da comunidade.", "INAPPROPRIATE_CONTENT")).
			Once()
		comment := model.Comment{Text: "discurso de ódio", Rating: 1}
		body, _ := json.Marshal(comment)
		req := httptest.NewRequest("POST", "/beers/1/comments", bytes.NewBuffer(body))
		req.SetPathValue("id", "1")
		rr := httptest.NewRecorder()

		controller.AddComment(rr, req)

		if status := rr.Code; status != http.StatusUnprocessableEntity {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnprocessableEntity)
		}
	})
}

// TestLikeComment tests the POST /beers/{id}/comments/{commentId}/like endpoint for liking a comment.
func TestLikeComment(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	// Like exige login (middleware.Auth). Injeta user_id no contexto.
	withUser := func(r *http.Request) *http.Request {
		return r.WithContext(middleware.WithUserID(r.Context(), "usr_1", ""))
	}

	// Test case 1: Successfully like a comment
	t.Run("Successful Like", func(t *testing.T) {
		mockBeerUsecase.On("LikeComment", mock.Anything, "1", "1", "usr_1", "").Return(nil).Once()
		req := withUser(httptest.NewRequest("POST", "/beers/1/comments/1/like", nil))
		rr := httptest.NewRecorder()

		req.SetPathValue("id", "1")
		req.SetPathValue("commentId", "1")
		controller.LikeComment(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// Test case 3: Already liked
	t.Run("Already Liked", func(t *testing.T) {
		req := withUser(httptest.NewRequest("POST", "/beers/1/comments/1/like", nil))
		rr := httptest.NewRecorder()

		req.SetPathValue("id", "1")
		req.SetPathValue("commentId", "1")
		mockBeerUsecase.LikeCommentFunc = func(ctx context.Context, beerID, commentID, userID, deviceID string) error {
			if userID == "usr_1" {
				return errors.NewAppError(400, "already liked", nil)
			}
			return nil
		}
		controller.LikeComment(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})
}

// TestDeleteComment tests the DELETE /beers/{id}/comments/{commentId} endpoint for deleting a comment.
func TestDeleteComment(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	// Test case 1: Successfully delete a comment
	t.Run("Successful Delete", func(t *testing.T) {
		mockBeerUsecase.On("DeleteComment", mock.Anything, "1", "1").Return(nil).Once()
		req := httptest.NewRequest("DELETE", "/beers/1/comments/1", nil)
		rr := httptest.NewRecorder()

		req.SetPathValue("id", "1")
		req.SetPathValue("commentId", "1")
		controller.DeleteComment(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// Test case 2: Comment not found
	t.Run("Comment Not Found", func(t *testing.T) {
		mockBeerUsecase.On("DeleteComment", mock.Anything, "1", "999").Return(errors.NewAppError(http.StatusNotFound, "comment not found", nil)).Once()
		req := httptest.NewRequest("DELETE", "/beers/1/comments/999", nil)
		rr := httptest.NewRecorder()

		req.SetPathValue("id", "1")
		req.SetPathValue("commentId", "999")
		controller.DeleteComment(rr, req)

		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
		}
	})
}

// TestCreateBeerValidationFailure valida que um payload inválido (ex: nome
// abaixo do mínimo) devolve 400 com mensagem de validação traduzida e LIMPA,
// sem o prefixo interno "Error 400: invalid beer:" (regressão do Bug 2).
func TestCreateBeerValidationFailure(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	abv := 5.0
	beer := model.Beer{
		Name:        "ab", // < min=3
		Style:       "IPA",
		Description: "desc",
		ImageUrl:    "https://x.com/a.jpg",
		Alcohol:     &abv,
		Taste:       "Doce",
		Aroma:       "Floral",
		Color:       "Clara",
		Body:        "Leve",
		Carbonation: "Baixa",
		Finish:      "Seco",
	}
	body, _ := json.Marshal(beer)
	req := httptest.NewRequest(http.MethodPost, "/beers", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	controller.CreateBeer(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	// O usecase NÃO deve ser chamado para um payload inválido.
	mockBeerUsecase.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	var resp struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body should be JSON, got %q: %v", rr.Body.String(), err)
	}
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.Status)
	}
	if strings.Contains(resp.Detail, "Error 400:") || strings.Contains(resp.Detail, "invalid beer:") {
		t.Fatalf("validation error leaked internal prefix: %q", resp.Detail)
	}
	if !strings.Contains(resp.Detail, "nome:") {
		t.Fatalf("expected translated field label in error, got %q", resp.Detail)
	}
}

// TestGetAllBeersRegressionNullSlice é um teste de regressão (filosofia
// table-driven) para o contrato de resposta de GET /api/v1/beers. Garante que
// uma lista vazia é serializada como "[]" e nunca como "null" (Bug Gap: o
// repositorio devolvia slice nil -> json null), e que o caminho de erro do
// usecase produz 500 sem vazar a causa interna.
func TestGetAllBeersRegressionNullSlice(t *testing.T) {
	tests := []struct {
		name       string
		beers      []model.Beer
		total      int
		repoErr    error
		wantStatus int
		wantBeers  string // substring esperada no body
	}{
		{
			name:       "lista vazia serializa como colchetes",
			beers:      []model.Beer{},
			total:      0,
			repoErr:    nil,
			wantStatus: http.StatusOK,
			wantBeers:  `"beers":[]`,
		},
		{
			name:       "nil slice nao pode produzir null",
			beers:      nil,
			total:      0,
			repoErr:    nil,
			wantStatus: http.StatusOK,
			wantBeers:  `"beers":[]`,
		},
		{
			name:       "uma cerveja presente",
			beers:      []model.Beer{{ID: "b1", Name: "IPA"}},
			total:      1,
			repoErr:    nil,
			wantStatus: http.StatusOK,
			wantBeers:  `"id":"b1"`,
		},
		{
			name:       "erro do repositorio preserva o status do AppError (503)",
			beers:      nil,
			total:      0,
			repoErr:    errors.NewAppError(503, "database is not ready", nil),
			wantStatus: http.StatusServiceUnavailable,
			wantBeers:  "application/problem+json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBeerUsecase.On("GetPaginated", mock.Anything, 1, 10).
				Return(tt.beers, tt.total, tt.repoErr).Once()

			controller := NewBeerController(mockBeerUsecase, mockLogger, nil)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/beers?page=1&pageSize=10", nil)
			rr := httptest.NewRecorder()

			controller.GetAllBeers(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if tt.wantBeers == "application/problem+json" {
				if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, tt.wantBeers) {
					t.Fatalf("Content-Type = %q, want %q (body=%s)", ct, tt.wantBeers, rr.Body.String())
				}
			} else if !strings.Contains(rr.Body.String(), tt.wantBeers) {
				t.Fatalf("body %q não contém %q", rr.Body.String(), tt.wantBeers)
			}
			// Regra de contrato: nunca expor null para a lista.
			if strings.Contains(rr.Body.String(), `"beers":null`) {
				t.Fatalf("contrato violado: beers serializado como null: %s", rr.Body.String())
			}
			mockBeerUsecase.AssertExpectations(t)
		})
	}
}

func TestDeleteBeer_NotFound(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	mockBeerUsecase.On("Delete", mock.Anything, "999").Return(errors.NewAppError(http.StatusNotFound, "beer not found", nil)).Once()

	req := httptest.NewRequest(http.MethodDelete, "/beers/999", nil)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()

	controller.DeleteBeer(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestUpdateBeer_ValidationFailure(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	beer := model.Beer{Name: "ab", Style: "IPA", Description: "desc", ImageUrl: "https://x.com/a.jpg", Alcohol: float64Ptr(5.0), Taste: "Doce", Aroma: "Floral", Color: "Clara", Body: "Leve", Carbonation: "Baixa", Finish: "Seco"}
	body, _ := json.Marshal(beer)
	req := httptest.NewRequest(http.MethodPut, "/beers/1", bytes.NewBuffer(body))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	controller.UpdateBeer(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	mockBeerUsecase.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestSearchBeers_AlcoholFilters(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	mockBeerUsecase.On("SearchBeers", mock.Anything, model.BeerFilters{
		Query:      "",
		Style:      "",
		Taste:      "",
		Page:       1,
		PageSize:   10,
		MinAlcohol: float64Ptr(4.0),
		MaxAlcohol: float64Ptr(6.0),
	}).Return([]model.Beer{{ID: "1", Name: "IPA"}}, 1, false, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers/search?minAlcohol=4.0&maxAlcohol=6.0&page=1&pageSize=10", nil)
	rr := httptest.NewRecorder()

	controller.SearchBeers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}
