package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"

	"github.com/stretchr/testify/mock"
)

func TestSearchBeers(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		setupMock  func()
		wantStatus int
		wantBody   string
	}{
		{
			name:  "success",
			query: "/api/v1/beers/search?q=IPA&page=1&pageSize=10",
			setupMock: func() {
				mockBeerUsecase.On("SearchBeers", mock.Anything, model.BeerFilters{
					Query:    "IPA",
					Page:     1,
					PageSize: 10,
				}).Return([]model.Beer{{ID: "1", Name: "IPA"}}, 1, false, nil).Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   `"beers"`,
		},
		{
			name:  "empty result",
			query: "/api/v1/beers/search?q=nonexistent",
			setupMock: func() {
				mockBeerUsecase.On("SearchBeers", mock.Anything, model.BeerFilters{
					Query:    "nonexistent",
					Page:     1,
					PageSize: 10,
				}).Return([]model.Beer{}, 0, false, nil).Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   `"beers":[]`,
		},
		{
			name:  "propagates error",
			query: "/api/v1/beers/search?q=IPA",
			setupMock: func() {
				mockBeerUsecase.On("SearchBeers", mock.Anything, model.BeerFilters{
					Query:    "IPA",
					Page:     1,
					PageSize: 10,
				}).Return([]model.Beer{}, 0, false, errors.NewAppError(500, "db error", nil)).Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `"code":"internal_server_error"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBeerUsecase = new(MockBeerUsecase)
			controller := NewBeerController(mockBeerUsecase, mockLogger, nil)
			tt.setupMock()

			req := httptest.NewRequest(http.MethodGet, tt.query, nil)
			rr := httptest.NewRecorder()
			controller.SearchBeers(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tt.wantBody) {
				t.Fatalf("body %q does not contain %q", rr.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestGetEnums(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/beers/enums", nil)
	rr := httptest.NewRecorder()
	controller.GetEnums(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var payload map[string][]string
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response should be valid JSON, got %q: %v", rr.Body.String(), err)
	}
	if len(payload) == 0 {
		t.Fatal("expected enums payload to have data")
	}

	etag := rr.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header to be set")
	}
}

func TestGetHomeFeed(t *testing.T) {
	mockBeerUsecase = new(MockBeerUsecase)
	controller := NewBeerController(mockBeerUsecase, mockLogger, nil)

	mockBeerUsecase.On("GetPaginated", mock.Anything, 1, 20).Return([]model.Beer{}, 0, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feed", nil)
	req.SetPathValue("cursor", "")
	rr := httptest.NewRecorder()
	controller.GetHomeFeed(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestSetPaginationLinks(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		page     int
		pageSize int
		total    int
		want     string
	}{
		{"first page", "/api/v1/beers", 1, 10, 25, `rel="next"`},
		{"middle page", "/api/v1/beers", 2, 10, 25, `rel="next"`},
		{"last page", "/api/v1/beers", 3, 10, 25, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tt.base, nil)
			setPaginationLinks(w, r, tt.page, tt.pageSize, tt.total, tt.base)
			link := w.Header().Get("Link")
			if tt.want == "" {
				if link != "" {
					t.Errorf("expected empty Link header, got %q", link)
				}
				return
			}
			if !strings.Contains(link, tt.want) {
				t.Errorf("expected Link header to contain %q, got %q", tt.want, link)
			}
		})
	}
}
