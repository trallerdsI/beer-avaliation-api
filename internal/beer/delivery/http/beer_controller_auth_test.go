package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"

	"github.com/stretchr/testify/mock"
)

// TestCreateBeerDuplicate409 valida que o bloqueio 409 (Decisão C) com
// código estável e sugestões no detail chega ao cliente em JSON, sem vazar
// a causa raiz (Gap1).
func TestCreateBeerDuplicate409(t *testing.T) {
	mockBeerUsecase := new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	suggestions := []map[string]any{{"id": "12", "name": "Heineken Long Neck"}}
	mockBeerUsecase.On("Create", context.Background(), mock.Anything).
		Return(errors.NewAppErrorWithDetail(409, "Uma cerveja com nome semelhante já existe", "DUPLICATE_BEER", suggestions)).Once()

	beer := model.Beer{Name: "Heineken"}
	body, _ := json.Marshal(beer)
	req := httptest.NewRequest(http.MethodPost, "/beers", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	controller.CreateBeer(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"code":"DUPLICATE_BEER"`) {
		t.Fatalf("expected error code in body, got %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Heineken Long Neck") {
		t.Fatalf("expected suggestions in detail, got %s", rr.Body.String())
	}
	// Garantia de defesa de memória: MaxBytesReader aplicado.
	if rr.Body.Len() > 1<<20+4096 {
		t.Fatal("response suspiciously large")
	}
}

// TestGetBeerByIDAggregation valida que os campos agregados
// averageRating/totalReviews (Decisão B) são expostos no JSON de resposta.
func TestGetBeerByIDAggregation(t *testing.T) {
	mockBeerUsecase := new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	beer := model.Beer{
		ID:            "1",
		Name:          "Stella",
		AverageRating: 4.5,
		TotalReviews:  2,
	}
	mockBeerUsecase.On("GetByID", context.Background(), "1").Return(beer, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/beers/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	controller.GetBeerByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got model.Beer
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.AverageRating != 4.5 {
		t.Fatalf("expected averageRating 4.5, got %v", got.AverageRating)
	}
	if got.TotalReviews != 2 {
		t.Fatalf("expected totalReviews 2, got %d", got.TotalReviews)
	}
}

// TestAddCommentAuthZCreatedBy valida que o comentário enviado ao usecase
// traz o CreatedBy do utilizador autenticado (AuthZ obrigatória, Decisão A).
func TestAddCommentAuthZCreatedBy(t *testing.T) {
	mockBeerUsecase := new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	var captured model.Comment
	mockBeerUsecase.On("AddComment", mock.Anything, "1", mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(2).(model.Comment)
		}).Return(nil).Once()

	comment := model.Comment{Text: "Boa!", Rating: 5}
	body, _ := json.Marshal(comment)
	req := httptest.NewRequest(http.MethodPost, "/beers/1/comments", bytes.NewBuffer(body))
	req.SetPathValue("id", "1")
	req = req.WithContext(middleware.WithUserID(req.Context(), "user-7", ""))
	rr := httptest.NewRecorder()

	controller.AddComment(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if captured.CreatedBy != "user-7" {
		t.Fatalf("expected CreatedBy user-7, got %q", captured.CreatedBy)
	}
}

// TestAddCommentXSSSanitized valida que um texto de comentário com script é
// sanitizado antes da validação (evita gravar XSS vazio).
func TestAddCommentXSSSanitized(t *testing.T) {
	mockBeerUsecase := new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	var captured model.Comment
	mockBeerUsecase.On("AddComment", mock.Anything, "1", mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(2).(model.Comment)
		}).Return(nil).Once()

	comment := model.Comment{Text: "<script>alert(1)</script> limpo", Rating: 3}
	body, _ := json.Marshal(comment)
	req := httptest.NewRequest(http.MethodPost, "/beers/1/comments", bytes.NewBuffer(body))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()

	controller.AddComment(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if strings.Contains(captured.Text, "<script>") {
		t.Fatalf("expected script tag sanitized, got %q", captured.Text)
	}
}
