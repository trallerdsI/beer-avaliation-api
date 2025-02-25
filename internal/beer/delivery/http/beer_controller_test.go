package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/usecase"
	"beer-review-app/pkg/errors"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// Create mock dependencies
var (
	mockLogger      = zap.NewNop()
	mockBeerUsecase = &MockBeerUsecase{} // Replace with mock struct
)

// MockBeerUsecase is a mock implementation of BeerUsecase
type MockBeerUsecase struct {
	usecase.BeerUsecase
	LikeCommentFunc func(ctx context.Context, beerID, commentID, deviceID string) error
}

func (m *MockBeerUsecase) AddComment(ctx context.Context, id string, comment model.Comment) error {
	if id == "1" {
		return nil
	}
	return errors.NewAppError(500, "failed to add comment", nil)
}

func (m *MockBeerUsecase) DeleteComment(ctx context.Context, beerID, commentID string) error {
	if beerID == "1" && commentID == "1" {
		return nil
	}
	return errors.NewAppError(500, "failed to delete comment", nil)
}

func (m *MockBeerUsecase) LikeComment(ctx context.Context, beerID, commentID, deviceID string) error {
	if m.LikeCommentFunc != nil {
		return m.LikeCommentFunc(ctx, beerID, commentID, deviceID)
	}
	if beerID == "1" && commentID == "1" && deviceID != "" {
		return nil
	}
	return errors.NewAppError(500, "failed to like comment", nil)
}

// TestGetBeer tests the GET /beers/{id} endpoint for retrieving a beer by its ID.
func TestGetBeer(t *testing.T) {
	// Test for a valid beer ID
	req, err := http.NewRequest("GET", "/beers/1", nil) // Create a new GET request for beer ID 1
	if err != nil {
		t.Fatal(err) // Fail the test if there is an error
	}
	rr := httptest.NewRecorder() // Create a ResponseRecorder to capture the response
	controller := NewBeerController(mockBeerUsecase, mockLogger)
	handler := http.HandlerFunc(controller.GetAllBeers)
	handler.ServeHTTP(rr, req) // Serve the HTTP request

	// Check the status code for a valid beer ID
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK) // Report an error if the status code is not OK
	}

	// Additional test case for non-existent beer ID
	req, err = http.NewRequest("GET", "/beers/999", nil) // Create a new GET request for a non-existent beer ID
	if err != nil {
		t.Fatal(err) // Fail the test if there is an error
	}
	rr = httptest.NewRecorder() // Reset the ResponseRecorder
	handler.ServeHTTP(rr, req)  // Serve the HTTP request

	// Check the status code for non-existent beer
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for non-existent beer: got %v want %v",
			status, http.StatusNotFound) // Report an error if the status code is not NotFound
	}
}

// TestCreateBeer tests the POST /beers endpoint for creating a new beer.
func TestCreateBeer(t *testing.T) {
	// Test for successful beer creation
	req, err := http.NewRequest("POST", "/beers", nil) // Create a new POST request to create a beer
	if err != nil {
		t.Fatal(err) // Fail the test if there is an error
	}
	rr := httptest.NewRecorder() // Create a ResponseRecorder to capture the response
	controller := NewBeerController(mockBeerUsecase, mockLogger)
	handler := http.HandlerFunc(controller.CreateBeer)
	handler.ServeHTTP(rr, req) // Serve the HTTP request

	// Check the status code for successful creation
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated) // Report an error if the status code is not Created
	}

	// Additional test case for invalid beer data
	req, err = http.NewRequest("POST", "/beers", nil) // Create a new POST request with invalid data
	if err != nil {
		t.Fatal(err) // Fail the test if there is an error
	}
	rr = httptest.NewRecorder() // Reset the ResponseRecorder
	handler.ServeHTTP(rr, req)  // Serve the HTTP request

	// Check the status code for invalid data
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid data: got %v want %v",
			status, http.StatusBadRequest) // Report an error if the status code is not BadRequest
	}
}

// TestUpdateBeer tests the PUT /beers/{id} endpoint for updating an existing beer.
func TestUpdateBeer(t *testing.T) {
	// Test for successful update
	req, err := http.NewRequest("PUT", "/beers/1", nil) // Create a new PUT request to update beer ID 1
	if err != nil {
		t.Fatal(err) // Fail the test if there is an error
	}
	rr := httptest.NewRecorder() // Create a ResponseRecorder to capture the response
	controller := NewBeerController(mockBeerUsecase, mockLogger)
	handler := http.HandlerFunc(controller.UpdateBeer)
	handler.ServeHTTP(rr, req) // Serve the HTTP request

	// Check the status code for successful update
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code for update: got %v want %v",
			status, http.StatusOK) // Report an error if the status code is not OK
	}
}

// TestDeleteBeer tests the DELETE /beers/{id} endpoint for deleting a beer.
func TestDeleteBeer(t *testing.T) {
	// Test for successful deletion
	req, err := http.NewRequest("DELETE", "/beers/1", nil) // Create a new DELETE request for beer ID 1
	if err != nil {
		t.Fatal(err) // Fail the test if there is an error
	}
	rr := httptest.NewRecorder() // Create a ResponseRecorder to capture the response
	controller := NewBeerController(mockBeerUsecase, mockLogger)
	handler := http.HandlerFunc(controller.DeleteBeer)
	handler.ServeHTTP(rr, req) // Serve the HTTP request

	// Check the status code for successful deletion
	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code for deletion: got %v want %v",
			status, http.StatusNoContent) // Report an error if the status code is not NoContent
	}
}

func TestAddComment(t *testing.T) {
	// Setup
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	// Test case 1: Valid comment
	t.Run("Valid Comment", func(t *testing.T) {
		comment := model.Comment{
			Text:     "Great beer!",
			Positive: true,
		}
		body, _ := json.Marshal(comment)
		req := httptest.NewRequest("POST", "/beers/1/comments", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		controller.AddComment(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// Test case 2: Missing text
	t.Run("Missing Text", func(t *testing.T) {
		comment := model.Comment{
			Positive: true,
		}
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
		comment := model.Comment{
			Text: "Great beer!",
		}
		body, _ := json.Marshal(comment)
		req := httptest.NewRequest("POST", "/beers/1/comments", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		controller.AddComment(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	// Test case 4: Positive field explicitly set to false
	t.Run("Positive Field False", func(t *testing.T) {
		comment := model.Comment{
			Text:     "Not great",
			Positive: false,
		}
		body, _ := json.Marshal(comment)
		req := httptest.NewRequest("POST", "/beers/1/comments", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		controller.AddComment(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// Test case 5: Positive field omitted from JSON
	t.Run("Positive Field Omitted", func(t *testing.T) {
		reqBody := []byte(`{"text": "Great beer!"}`)
		req := httptest.NewRequest("POST", "/beers/1/comments", bytes.NewBuffer(reqBody))
		rr := httptest.NewRecorder()

		controller.AddComment(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})
}

func TestLikeComment(t *testing.T) {
	// Setup
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	// Test case 1: Successfully like a comment
	t.Run("Successful Like", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/beers/1/comments/1/like", nil)
		req.Header.Set("X-Device-ID", "device123") // Add device ID header
		rr := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{
			"id":        "1",
			"commentId": "1",
		})

		controller.LikeComment(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// Test case 2: Missing device ID
	t.Run("Missing Device ID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/beers/1/comments/1/like", nil)
		rr := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{
			"id":        "1",
			"commentId": "1",
		})

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

		req = mux.SetURLVars(req, map[string]string{
			"id":        "1",
			"commentId": "1",
		})

		// Mock usecase to return "already liked" error
		mockBeerUsecase := &MockBeerUsecase{}
		mockBeerUsecase.LikeCommentFunc = func(ctx context.Context, beerID, commentID, deviceID string) error {
			if deviceID == "device-already-liked" {
				return errors.NewAppError(400, "already liked", nil)
			}
			return nil
		}
		controller = NewBeerController(mockBeerUsecase, mockLogger)

		controller.LikeComment(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})
}

func TestDeleteComment(t *testing.T) {
	// Setup
	controller := NewBeerController(mockBeerUsecase, mockLogger)

	// Test case 1: Successfully delete a comment
	t.Run("Successful Delete", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/beers/1/comments/1", nil)
		rr := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{
			"id":        "1",
			"commentId": "1",
		})

		controller.DeleteComment(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// Test case 2: Comment not found
	t.Run("Comment Not Found", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/beers/1/comments/999", nil)
		rr := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{
			"id":        "1",
			"commentId": "999",
		})

		controller.DeleteComment(rr, req)

		if status := rr.Code; status != http.StatusInternalServerError {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
		}
	})
}
