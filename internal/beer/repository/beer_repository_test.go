package repository

import (
	"context"
	"sync"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"
)

// InMemoryBeerRepository é uma implementação em memória para testes.
type InMemoryBeerRepository struct {
	beers []model.Beer
	mutex sync.RWMutex
}

func NewInMemoryBeerRepository() *InMemoryBeerRepository {
	return &InMemoryBeerRepository{beers: []model.Beer{}}
}

func (r *InMemoryBeerRepository) Create(ctx context.Context, beer *model.Beer) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.beers = append(r.beers, *beer)
	return nil
}

func (r *InMemoryBeerRepository) GetByID(ctx context.Context, id string) (model.Beer, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, beer := range r.beers {
		if beer.ID == id {
			return beer, nil
		}
	}
	return model.Beer{}, errors.NewAppError(404, "beer not found", nil)
}

func (r *InMemoryBeerRepository) GetAll(ctx context.Context) ([]model.Beer, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	out := make([]model.Beer, len(r.beers))
	copy(out, r.beers)
	return out, nil
}

func (r *InMemoryBeerRepository) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	start := (page - 1) * pageSize
	total := len(r.beers)
	if start >= total || pageSize <= 0 {
		return []model.Beer{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	seg := r.beers[start:end]
	out := make([]model.Beer, len(seg))
	copy(out, seg)
	return out, total, nil
}

func (r *InMemoryBeerRepository) Update(ctx context.Context, id string, beer model.Beer) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i, b := range r.beers {
		if b.ID == id {
			r.beers[i] = beer
			return nil
		}
	}
	return errors.NewAppError(404, "beer not found", nil)
}

func (r *InMemoryBeerRepository) Delete(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i, b := range r.beers {
		if b.ID == id {
			r.beers = append(r.beers[:i], r.beers[i+1:]...)
			return nil
		}
	}
	return errors.NewAppError(404, "beer not found", nil)
}

func (r *InMemoryBeerRepository) AddComment(ctx context.Context, id string, comment model.Comment) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i, beer := range r.beers {
		if beer.ID == id {
			r.beers[i].Comments = append(r.beers[i].Comments, comment)
			return nil
		}
	}
	return errors.NewAppError(404, "beer not found", nil)
}

func (r *InMemoryBeerRepository) DeleteComment(ctx context.Context, id string, commentID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i, beer := range r.beers {
		if beer.ID == id {
			for j, c := range beer.Comments {
				if c.ID == commentID {
					r.beers[i].Comments = append(beer.Comments[:j], beer.Comments[j+1:]...)
					return nil
				}
			}
			return errors.NewAppError(404, "comment not found", nil)
		}
	}
	return errors.NewAppError(404, "beer not found", nil)
}

func (r *InMemoryBeerRepository) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error) {
	return nil, 0, errors.NewAppError(501, "not implemented in memory repo", nil)
}

func (r *InMemoryBeerRepository) GetAdminStats(ctx context.Context) (*AdminStats, error) {
	return &AdminStats{}, nil
}

func (r *InMemoryBeerRepository) GetUserStats(ctx context.Context, userID string) (*UserStats, error) {
	return &UserStats{UserID: userID}, nil
}

// TestSetRatingAggregates valida a agregação de averageRating/totalReviews
// (Decisão B) a partir dos ratings dos comentários, incluindo notas fora do
// intervalo [1,5] que devem ser ignoradas no cálculo.
func TestSetRatingAggregates(t *testing.T) {
	cases := []struct {
		name        string
		comments    []model.Comment
		wantAvg     float64
		wantReviews int
	}{
		{
			name:        "sem comentários",
			comments:    nil,
			wantAvg:     0,
			wantReviews: 0,
		},
		{
			name:        "média simples",
			comments:    []model.Comment{{Rating: 4}, {Rating: 4}},
			wantAvg:     4,
			wantReviews: 2,
		},
		{
			name:        "média com 1 casa decimal",
			comments:    []model.Comment{{Rating: 4}, {Rating: 5}},
			wantAvg:     4.5,
			wantReviews: 2,
		},
		{
			name:        "ignora ratings fora de [1,5]",
			comments:    []model.Comment{{Rating: 0}, {Rating: 9}, {Rating: 3}},
			wantAvg:     3,
			wantReviews: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := &model.Beer{Comments: tc.comments}
			setRatingAggregates(b)
			if b.AverageRating != tc.wantAvg {
				t.Fatalf("averageRating: got %v, want %v", b.AverageRating, tc.wantAvg)
			}
			if b.TotalReviews != tc.wantReviews {
				t.Fatalf("totalReviews: got %d, want %d", b.TotalReviews, tc.wantReviews)
			}
		})
	}
}

// TestInMemoryBeerRepository cobre o ciclo CRUD em memória (sem DB), útil
// para validar o usecase de forma determinística em testes e serverless.
func TestInMemoryBeerRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryBeerRepository()

	beer := model.Beer{ID: "1", Name: "Heineken", Comments: []model.Comment{}}
	if err := repo.Create(ctx, &beer); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Heineken" {
		t.Fatalf("expected Heineken, got %q", got.Name)
	}

	// GetByID de cerveja inexistente → 404 AppError.
	if _, err := repo.GetByID(ctx, "999"); err == nil {
		t.Fatal("expected 404 for missing beer")
	}

	// Paginação: uma cerveja, page 1 size 10.
	list, total, err := repo.GetPaginated(ctx, 1, 10)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("GetPaginated: total=%d len=%d err=%v", total, len(list), err)
	}

	// Delete.
	if err := repo.Delete(ctx, "1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, "1"); err == nil {
		t.Fatal("expected beer to be gone after delete")
	}
}

// TestInMemoryAddDeleteComment valida o ciclo de comentários em memória,
// incluindo o caso de comentário inexistente (404) e cerveja inexistente.
func TestInMemoryAddDeleteComment(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryBeerRepository()
	beer := model.Beer{ID: "1", Name: "X", Comments: []model.Comment{}}
	if err := repo.Create(ctx, &beer); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.AddComment(ctx, "1", model.Comment{ID: "c1", Text: "oi"}); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	got, _ := repo.GetByID(ctx, "1")
	if len(got.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(got.Comments))
	}

	if err := repo.DeleteComment(ctx, "1", "c1"); err != nil {
		t.Fatalf("DeleteComment: %v", err)
	}
	got, _ = repo.GetByID(ctx, "1")
	if len(got.Comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(got.Comments))
	}

	// Comentário inexistente → 404.
	if err := repo.DeleteComment(ctx, "1", "nope"); err == nil {
		t.Fatal("expected 404 for missing comment")
	}
	// Cerveja inexistente → 404.
	if err := repo.AddComment(ctx, "999", model.Comment{ID: "c2"}); err == nil {
		t.Fatal("expected 404 for missing beer")
	}
}

// TestUnmarshalComments valida o fallback seguro para JSON inválido/vazio.
func TestUnmarshalComments(t *testing.T) {
	if got := unmarshalComments(nil); len(got) != 0 {
		t.Fatalf("expected empty for nil, got %d", len(got))
	}
	if got := unmarshalComments([]byte("[]")); len(got) != 0 {
		t.Fatalf("expected empty for empty array, got %d", len(got))
	}
	// JSON corrompido → fallback a lista vazia (não panics).
	if got := unmarshalComments([]byte("not-json")); len(got) != 0 {
		t.Fatalf("expected empty for invalid json, got %d", len(got))
	}
	valid := []byte(`[{"id":"c1","text":"bom","rating":5}]`)
	got := unmarshalComments(valid)
	if len(got) != 1 || got[0].ID != "c1" {
		t.Fatalf("unexpected parsed comments: %#v", got)
	}
}

// TestUnmarshalMedia valida o fallback seguro para JSON inválido/vazio.
func TestUnmarshalMedia(t *testing.T) {
	if got := unmarshalMedia(nil); len(got) != 0 {
		t.Fatalf("expected empty for nil, got %d", len(got))
	}
	if got := unmarshalMedia([]byte("[]")); len(got) != 0 {
		t.Fatalf("expected empty for empty array, got %d", len(got))
	}
	if got := unmarshalMedia([]byte("not-json")); len(got) != 0 {
		t.Fatalf("expected empty for invalid json, got %d", len(got))
	}
	valid := []byte(`[{"url":"http://img","type":"image/jpeg"}]`)
	got := unmarshalMedia(valid)
	if len(got) != 1 || got[0].URL != "http://img" {
		t.Fatalf("unexpected parsed media: %#v", got)
	}
}

// TestInMemoryGetAll valida a listagem completa.
func TestInMemoryGetAll(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryBeerRepository()
	if err := repo.Create(ctx, &model.Beer{ID: "1", Name: "A"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, &model.Beer{ID: "2", Name: "B"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	all, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("GetAll len: got %d, want 2", len(all))
	}
}
