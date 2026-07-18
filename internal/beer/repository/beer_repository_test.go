package repository

import (
	"context"
	stderrors "errors"
	"testing"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"
)

// TestUnavailableBeerRepository garante que o fallback offline retorna
// ErrDatabaseUnavailable (503) em todas as operações, mantendo o servidor
// de pé para rotas de diagnóstico quando a DB não está acessível.
func TestUnavailableBeerRepository(t *testing.T) {
	repo := NewUnavailableBeerRepository()
	ctx := context.Background()

	if _, err := repo.GetAll(ctx); err == nil {
		t.Fatal("GetAll: expected error")
	}
	if err := repo.Create(ctx, &model.Beer{ID: "1", Name: "x"}); err == nil {
		t.Fatal("Create: expected error")
	}
	if _, _, err := repo.GetPaginated(ctx, 1, 10); err == nil {
		t.Fatal("GetPaginated: expected error")
	}
	if _, err := repo.GetByID(ctx, "1"); err == nil {
		t.Fatal("GetByID: expected error")
	}
	if err := repo.Update(ctx, "1", model.Beer{ID: "1", Name: "x"}); err == nil {
		t.Fatal("Update: expected error")
	}
	if err := repo.Delete(ctx, "1"); err == nil {
		t.Fatal("Delete: expected error")
	}
	if err := repo.AddComment(ctx, "1", model.Comment{ID: "c1"}); err == nil {
		t.Fatal("AddComment: expected error")
	}
	if err := repo.DeleteComment(ctx, "1", "c1"); err == nil {
		t.Fatal("DeleteComment: expected error")
	}
	if _, _, err := repo.SearchBeers(ctx, model.BeerFilters{Page: 1, PageSize: 10}); err == nil {
		t.Fatal("SearchBeers: expected error")
	}

	// O erro deve ser inspecionável via errors.As/Is.
	var appErr *errors.AppError
	if _, err := repo.GetByID(ctx, "1"); !stderrors.As(err, &appErr) {
		t.Fatal("expected *errors.AppError")
	}
	if appErr.Code != 503 {
		t.Fatalf("expected 503, got %d", appErr.Code)
	}
	if !stderrors.Is(appErr, errors.ErrDatabaseUnavailable) {
		t.Fatal("expected ErrDatabaseUnavailable")
	}
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
