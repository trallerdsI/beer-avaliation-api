package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/middleware"
)

type fuzzBeerUsecase struct{}

func (f *fuzzBeerUsecase) GetAll(ctx context.Context) ([]model.Beer, error) {
	return nil, nil
}
func (f *fuzzBeerUsecase) Create(ctx context.Context, beer *model.Beer) error {
	return nil
}
func (f *fuzzBeerUsecase) GetByID(ctx context.Context, id string) (model.Beer, error) {
	return model.Beer{}, nil
}
func (f *fuzzBeerUsecase) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	return nil, 0, nil
}
func (f *fuzzBeerUsecase) Update(ctx context.Context, id string, beer model.Beer) error {
	return nil
}
func (f *fuzzBeerUsecase) Delete(ctx context.Context, id string) error {
	return nil
}
func (f *fuzzBeerUsecase) AddComment(ctx context.Context, id string, comment model.Comment) error {
	return nil
}
func (f *fuzzBeerUsecase) DeleteComment(ctx context.Context, beerID, commentID string) error {
	return nil
}
func (f *fuzzBeerUsecase) LikeComment(ctx context.Context, beerID, commentID, userID, deviceID string) error {
	return nil
}
func (f *fuzzBeerUsecase) AddMedia(ctx context.Context, id string, item model.MediaItem) ([]model.MediaItem, error) {
	return nil, nil
}
func (f *fuzzBeerUsecase) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, bool, error) {
	return nil, 0, false, nil
}
func (f *fuzzBeerUsecase) ListBeerEvents(ctx context.Context, beerID string, since time.Time) ([]model.BeerEvent, error) {
	return nil, nil
}

func FuzzCreateBeerHandler(f *testing.F) {
	f.Add(`{"name":"IPA","style":"Ale","alcohol":5.5,"taste":"Doce","aroma":"Floral","color":"Clara","body":"Leve","carbonation":"Baixa","finish":"Seco"}`)
	f.Add(`{}`)
	f.Add(`{"name":""}`)
	f.Add(`{"alcohol":999}`)
	f.Add(`{"name":"<script>alert(1)</script>"}`)

	controller := NewBeerController(&fuzzBeerUsecase{}, mockLogger, nil, nil)

	f.Fuzz(func(t *testing.T, data string) {
		body := bytes.NewReader([]byte(data))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/beers", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithUserID(req.Context(), "user-1", ""))

		w := httptest.NewRecorder()
		controller.CreateBeer(w, req)
	})
}

func FuzzUpdateBeerHandler(f *testing.F) {
	f.Add(`{"name":"IPA Updated","style":"Ale","alcohol":5.5,"taste":"Doce","aroma":"Floral","color":"Clara","body":"Leve","carbonation":"Baixa","finish":"Seco"}`)
	f.Add(`{}`)
	f.Add(`{"name":""}`)
	f.Add(`{"alcohol":999}`)

	controller := NewBeerController(&fuzzBeerUsecase{}, mockLogger, nil, nil)

	f.Fuzz(func(t *testing.T, data string) {
		body := bytes.NewReader([]byte(data))
		req := httptest.NewRequest(http.MethodPut, "/api/v1/beers/beer-1", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithUserID(req.Context(), "user-1", ""))

		w := httptest.NewRecorder()
		controller.UpdateBeer(w, req)
	})
}

func FuzzAddCommentHandler(f *testing.F) {
	f.Add(`{"text":"Great beer!","rating":5}`)
	f.Add(`{}`)
	f.Add(`{"text":""}`)
	f.Add(`{"rating":-1}`)
	f.Add(`{"text":"<script>alert(1)</script>","rating":5}`)

	controller := NewBeerController(&fuzzBeerUsecase{}, mockLogger, nil, nil)

	f.Fuzz(func(t *testing.T, data string) {
		body := bytes.NewReader([]byte(data))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/beers/beer-1/comments", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		controller.AddComment(w, req)
	})
}

func FuzzDecodeBeerJSON(f *testing.F) {
	f.Add(`{"name":"IPA","style":"Ale","alcohol":5.5,"taste":"Doce","aroma":"Floral","color":"Clara","body":"Leve","carbonation":"Baixa","finish":"Seco"}`)
	f.Add(`{"name":"","style":"","alcohol":0}`)
	f.Add(`{"comments":[{"id":"c1","text":"<script>alert(1)</script>","rating":5}]}`)
	f.Add(`{"media":[{"url":"http://example.com/img.jpg","type":"image/jpeg","size":1024}]}`)
	f.Add(`{"taste":"Invalid","aroma":"Invalid","color":"Invalid","body":"Invalid","carbonation":"Invalid","finish":"Invalid"}`)

	f.Fuzz(func(t *testing.T, data string) {
		var b model.Beer
		if err := json.Unmarshal([]byte(data), &b); err != nil {
			return
		}
		_, _ = json.Marshal(b)
	})
}

func FuzzDecodeCommentJSON(f *testing.F) {
	f.Add(`{"id":"c1","text":"Great beer!","rating":5,"likes":0,"likedBy":[],"createdBy":"user-1","createdAt":"2024-01-01T00:00:00Z"}`)
	f.Add(`{"id":"","text":"","rating":0}`)
	f.Add(`{"rating":5}`)
	f.Add(`{"text":"<script>alert(1)</script>","rating":999}`)

	f.Fuzz(func(t *testing.T, data string) {
		var c model.Comment
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			return
		}
		_, _ = json.Marshal(c)
	})
}
