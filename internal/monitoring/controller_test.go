package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
func (f *fakeBeerUsecase) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, bool, error) {
	return nil, 0, false, nil
}
func (f *fakeBeerUsecase) ListBeerEvents(ctx context.Context, beerID string, since time.Time) ([]model.BeerEvent, error) {
	return nil, nil
}

func TestHealthCheckNilDB(t *testing.T) {
	c := NewMonitoringController(&fakeBeerUsecase{}, nil, nil, nil, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	c.HealthCheck(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when db is nil, got %d", w.Code)
	}
}

func TestHealthCheckDBError(t *testing.T) {
	dbErr := errors.NewAppError(500, "db down", nil)
	c := NewMonitoringController(&fakeBeerUsecase{}, nil, nil, nil, nil, dbErr)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	c.HealthCheck(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when dbErr is set, got %d", w.Code)
	}
}

func TestGetStatsRepoNil(t *testing.T) {
	c := NewMonitoringController(&fakeBeerUsecase{}, nil, nil, nil, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)

	c.GetStats(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when repo is nil, got %d", w.Code)
	}
}

func TestGetAdminStatsForbidden(t *testing.T) {
	c := NewMonitoringController(&fakeBeerUsecase{}, nil, nil, nil, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)

	c.GetAdminStats(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when not admin, got %d", w.Code)
	}
}

func TestGetUserStatsUnauthorized(t *testing.T) {
	c := NewMonitoringController(&fakeBeerUsecase{}, nil, nil, nil, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/stats", nil)

	c.GetUserStats(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when not authenticated, got %d", w.Code)
	}
}
