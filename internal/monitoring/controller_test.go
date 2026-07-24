package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"
)

type fakeBeerUsecase struct {
	getPaginated func(ctx context.Context, page, pageSize int) ([]model.Beer, int, error)
}

func (f *fakeBeerUsecase) GetAll(ctx context.Context) ([]model.Beer, error) {
	return nil, nil
}
func (f *fakeBeerUsecase) Create(ctx context.Context, beer *model.Beer) error {
	return nil
}
func (f *fakeBeerUsecase) GetByID(ctx context.Context, id string) (model.Beer, error) {
	return model.Beer{}, nil
}
func (f *fakeBeerUsecase) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	return f.getPaginated(ctx, page, pageSize)
}
func (f *fakeBeerUsecase) Update(ctx context.Context, id string, beer model.Beer) error {
	return nil
}
func (f *fakeBeerUsecase) Delete(ctx context.Context, id string) error {
	return nil
}
func (f *fakeBeerUsecase) AddComment(ctx context.Context, id string, comment model.Comment) error {
	return nil
}
func (f *fakeBeerUsecase) DeleteComment(ctx context.Context, id string, commentID string) error {
	return nil
}
func (f *fakeBeerUsecase) LikeComment(ctx context.Context, beerID, commentID, userID, deviceID string) error {
	return nil
}
func (f *fakeBeerUsecase) AddMedia(ctx context.Context, id string, item model.MediaItem) ([]model.MediaItem, error) {
	return nil, nil
}
func (f *fakeBeerUsecase) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error) {
	return nil, 0, nil
}

func TestHealthCheckNilDB(t *testing.T) {
	c := NewMonitoringController(&fakeBeerUsecase{}, nil, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	c.HealthCheck(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when db is nil, got %d", w.Code)
	}
}

func TestHealthCheckDBError(t *testing.T) {
	dbErr := errors.NewAppError(500, "db down", nil)
	c := NewMonitoringController(&fakeBeerUsecase{}, nil, nil, dbErr)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	c.HealthCheck(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when dbErr is set, got %d", w.Code)
	}
}

func TestGetStatsUsecaseError(t *testing.T) {
	c := NewMonitoringController(&fakeBeerUsecase{
		getPaginated: func(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
			return nil, 0, errors.NewAppError(500, "db error", nil)
		},
	}, nil, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)

	c.GetStats(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on stats error, got %d", w.Code)
	}
}
