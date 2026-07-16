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
	if err := repo.Create(ctx, model.Beer{ID: "1", Name: "x"}); err == nil {
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
