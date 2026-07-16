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
	LikeCommentFunc func(ctx context.Context, beerID, commentID, deviceID string) error
	mock.Mock
}

func (m *MockBeerUsecase) GetAll(ctx context.Context) ([]model.Beer, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Beer), args.Error(1)
}

func (m *MockBeerUsecase) Create(ctx context.Context, beer model.Beer) error {
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

func (m *MockBeerUsecase) LikeComment(ctx context.Context, beerID, commentID, deviceID string) error {
	if m.LikeCommentFunc != nil {
		return m.LikeCommentFunc(ctx, beerID, commentID, deviceID)
	}
	args := m.Called(ctx, beerID, commentID, deviceID)
	return args.Error(0)
}

func (m *MockBeerUsecase) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]model.Beer), args.Int(1), args.Error(2)
}

func (m *MockBeerUsecase) GetByID(ctx context.Context, id string) (model.Beer, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.Beer), args.Error(1)
}

// TestGetBeer tests the GET /beers/{id} endpoint for retrieving a beer by its ID.
func TestGetBeer(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

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
	mockBeerUsecase.On("GetByID", context.Background(), "99").Return(model.Beer{}, errors.NewAppError(0, "not found", nil))

	// Request for non-existent beer ID 99
	rr = makeRequest("/beers/99")
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for non-existent beer: got %v want %v", status, http.StatusNotFound)
	}
}

// TestCreateBeer tests the POST /beers endpoint for creating a new beer.
func TestCreateBeer(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	// Test for successful beer creation
	mockBeerUsecase.On("Create", context.Background(), mock.Anything).Return(nil).Once()
	beer := model.Beer{Name: "Test Beer", Style: "IPA"}
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
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	// Test for successful update
	mockBeerUsecase.On("Update", context.Background(), "1", mock.Anything).Return(nil).Once()
	beer := model.Beer{Name: "Updated Beer", Style: "IPA"}
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
	controller := NewBeerController(mockBeerUsecase, mockLogger)

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
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	// Test case 1: Valid comment
	t.Run("Valid Comment", func(t *testing.T) {
		mockBeerUsecase.On("AddComment", context.Background(), "1", mock.Anything).Return(nil).Once()
		comment := model.Comment{Text: "Great beer!", Positive: true}
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
		comment := model.Comment{Positive: true}
		body, _ := json.Marshal(comment)
		req := httptest.NewRequest("POST", "/beers/1/comments", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		controller.AddComment(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	// Test case 3: Missing positive/negative status
	t.Run("Missing Positive Status", func(t *testing.T) {
		comment := model.Comment{Text: "Great beer!"}
		body, _ := json.Marshal(comment)
		req := httptest.NewRequest("POST", "/beers/1/comments", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		controller.AddComment(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})
}

// TestLikeComment tests the POST /beers/{id}/comments/{commentId}/like endpoint for liking a comment.
func TestLikeComment(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	// Test case 1: Successfully like a comment
	t.Run("Successful Like", func(t *testing.T) {
		mockBeerUsecase.On("LikeComment", mock.Anything, "1", "1", "device123").Return(nil).Once()
		req := httptest.NewRequest("POST", "/beers/1/comments/1/like", nil)
		req.Header.Set("X-Device-ID", "device123")
		rr := httptest.NewRecorder()

		req.SetPathValue("id", "1")
		req.SetPathValue("commentId", "1")
		controller.LikeComment(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// Test case 2: Missing device ID
	t.Run("Missing Device ID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/beers/1/comments/1/like", nil)
		rr := httptest.NewRecorder()

		req.SetPathValue("id", "1")
		req.SetPathValue("commentId", "1")
		controller.LikeComment(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	// Test case 3: Already liked by device
	t.Run("Already Liked", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/beers/1/comments/1/like", nil)
		req.Header.Set("X-Device-ID", "device-already-liked")
		rr := httptest.NewRecorder()

		req.SetPathValue("id", "1")
		req.SetPathValue("commentId", "1")
		mockBeerUsecase.LikeCommentFunc = func(ctx context.Context, beerID, commentID, deviceID string) error {
			if deviceID == "device-already-liked" {
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
	controller := NewBeerController(mockBeerUsecase, mockLogger)

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
