package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"beer-review-app/internal/user/model"
)

func TestPostgresUserRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definida; pulando teste de integração")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("falha ao conectar DB de teste: %v", err)
	}
	defer db.Close()

	repo, err := NewPostgresUserRepository(db)
	if err != nil {
		t.Fatalf("falha ao criar repo: %v", err)
	}

	ctx := context.Background()
	user := model.User{
		ID:       "test-user-1",
		Username: "testuser",
		Email:    "test@example.com",
		Password: "hashed",
		Role:     model.RoleUser,
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.Username != "testuser" {
		t.Fatalf("Username: got %q, want testuser", got.Username)
	}

	got, err = repo.GetByID(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != "test@example.com" {
		t.Fatalf("Email: got %q, want test@example.com", got.Email)
	}

	if err := repo.Delete(ctx, "test-user-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.GetByID(ctx, "test-user-1")
	if err == nil {
		t.Fatal("expected 404 after delete")
	}
}
