package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"beer-review-app/internal/beer/model"
	"beer-review-app/internal/beer/repository"
	usermodel "beer-review-app/internal/user/model"
	userRepository "beer-review-app/internal/user/repository"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
)

type fakeBeerRepo struct {
	stats *repository.AdminStats
	err   error
}

func (f *fakeBeerRepo) GetAll(ctx context.Context) ([]model.Beer, error) {
	return nil, nil
}
func (f *fakeBeerRepo) Create(ctx context.Context, beer *model.Beer) error {
	return nil
}
func (f *fakeBeerRepo) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	return nil, 0, nil
}
func (f *fakeBeerRepo) GetByID(ctx context.Context, id string) (model.Beer, error) {
	return model.Beer{}, nil
}
func (f *fakeBeerRepo) Update(ctx context.Context, id string, beer model.Beer) error {
	return nil
}
func (f *fakeBeerRepo) Delete(ctx context.Context, id string) error {
	return nil
}
func (f *fakeBeerRepo) AddComment(ctx context.Context, id string, comment model.Comment) error {
	return nil
}
func (f *fakeBeerRepo) DeleteComment(ctx context.Context, id string, commentID string) error {
	return nil
}
func (f *fakeBeerRepo) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, bool, error) {
	return nil, 0, false, nil
}
func (f *fakeBeerRepo) GetAdminStats(ctx context.Context) (*repository.AdminStats, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.stats, nil
}
func (f *fakeBeerRepo) GetUserStats(ctx context.Context, userID string) (*repository.UserStats, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &repository.UserStats{
		UserID:         userID,
		BeersReviewed:  5,
		TotalComments:  5,
		LikesGiven:     10,
		LikesReceived:  15,
		PositiveRatio:  80.0,
		FavoriteStyles: []repository.StyleCount{{Style: "IPA", Count: 3}},
		TopAromaNotes:  "Citrus",
		BeersAdded:     2,
	}, nil
}

func (f *fakeBeerRepo) ExecInTx(ctx context.Context, fn func(ctx context.Context, txRepo repository.BeerRepository) error) error {
	return fn(ctx, f)
}

type fakeUserRepo struct {
	memberSince time.Time
	err         error
}

func (f *fakeUserRepo) Create(ctx context.Context, user usermodel.User) error {
	return nil
}
func (f *fakeUserRepo) GetByID(ctx context.Context, id string) (usermodel.User, error) {
	return usermodel.User{}, nil
}
func (f *fakeUserRepo) GetByEmail(ctx context.Context, email string) (usermodel.User, error) {
	return usermodel.User{}, nil
}
func (f *fakeUserRepo) GetByExternal(ctx context.Context, provider, externalSub string) (usermodel.User, error) {
	return usermodel.User{}, nil
}
func (f *fakeUserRepo) UpsertByExternal(ctx context.Context, user usermodel.User) (usermodel.User, error) {
	return usermodel.User{}, nil
}
func (f *fakeUserRepo) Update(ctx context.Context, id string, user usermodel.User) error {
	return nil
}
func (f *fakeUserRepo) Delete(ctx context.Context, id string) error {
	return nil
}
func (f *fakeUserRepo) List(ctx context.Context, page, pageSize int) ([]usermodel.User, int, error) {
	return nil, 0, nil
}
func (f *fakeUserRepo) CreatePushSubscription(ctx context.Context, sub usermodel.PushSubscription) error {
	return nil
}
func (f *fakeUserRepo) ListPushSubscriptions(ctx context.Context, userID string) ([]usermodel.PushSubscription, error) {
	return nil, nil
}
func (f *fakeUserRepo) DeletePushSubscription(ctx context.Context, id string) error {
	return nil
}
func (f *fakeUserRepo) DeletePushSubscriptionByEndpoint(ctx context.Context, userID, endpoint string) error {
	return nil
}
func (f *fakeUserRepo) GetMemberSince(ctx context.Context, userID string) (time.Time, error) {
	return f.memberSince, f.err
}

func (f *fakeUserRepo) ExecInTx(ctx context.Context, fn func(ctx context.Context, txRepo userRepository.UserRepository) error) error {
	return fn(ctx, f)
}

func TestGetAdminStatsSuccess(t *testing.T) {
	stats := &repository.AdminStats{
		TotalUsers: 100,
		TotalBeers: 50,
		TopStyles:  []repository.StyleCount{{Style: "IPA", Count: 20}},
		TotalLikes: 300,
	}
	c := NewMonitoringController(&fakeBeerUsecase{}, &fakeBeerRepo{stats: stats}, &fakeUserRepo{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	ctx := middleware.WithUserID(req.Context(), "user1", "admin")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	c.GetAdminStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), `"total":100`) {
		t.Fatalf("expected total users in response: %s", w.Body.String())
	}
}

func TestGetAdminStatsRepoError(t *testing.T) {
	c := NewMonitoringController(&fakeBeerUsecase{}, &fakeBeerRepo{err: errors.ErrDatabaseUnavailable}, &fakeUserRepo{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	ctx := middleware.WithUserID(req.Context(), "user1", "admin")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	c.GetAdminStats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on repo error, got %d", w.Code)
	}
}

func TestGetUserStatsSuccess(t *testing.T) {
	beerRepo := &fakeBeerRepo{}
	userRepo := &fakeUserRepo{memberSince: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	c := NewMonitoringController(&fakeBeerUsecase{}, beerRepo, userRepo, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/stats", nil)
	ctx := middleware.WithUserID(req.Context(), "user1", "user")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	c.GetUserStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"user_id":"user1"`) {
		t.Fatalf("expected user_id in response: %s", w.Body.String())
	}
}

func TestGetUserStatsRepoError(t *testing.T) {
	beerRepo := &fakeBeerRepo{err: errors.ErrDatabaseUnavailable}
	userRepo := &fakeUserRepo{}
	c := NewMonitoringController(&fakeBeerUsecase{}, beerRepo, userRepo, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/stats", nil)
	ctx := middleware.WithUserID(req.Context(), "user1", "user")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	c.GetUserStats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on repo error, got %d", w.Code)
	}
}
