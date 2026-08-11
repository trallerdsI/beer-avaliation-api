package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"beer-review-app/internal/user/model"
	"beer-review-app/pkg/uuid"
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
	userID := uuid.MustNewV7()
	now := time.Now().UTC().Format(time.RFC3339)
	user := model.User{
		ID:       userID,
		Username: "testuser-" + userID[:8],
		Email:    "test-" + userID[:8] + "@example.com",
		Password: "hashed",
		Role:     model.RoleUser,
		Created:  now,
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.Username != user.Username {
		t.Fatalf("Username: got %q, want %s", got.Username, user.Username)
	}

	got, err = repo.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != user.Email {
		t.Fatalf("Email: got %q, want %s", got.Email, user.Email)
	}

	if err := repo.Delete(ctx, userID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.GetByID(ctx, userID)
	if err == nil {
		t.Fatal("expected 404 after delete")
	}
}

func TestPostgresUserRepositoryIntegrationGetMemberSince(t *testing.T) {
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
	userID := uuid.MustNewV7()
	created := time.Now().UTC().AddDate(0, 0, -10)
	user := model.User{
		ID:       userID,
		Username: "membertest-" + userID[:8],
		Email:    "member-" + userID[:8] + "@example.com",
		Password: "hashed",
		Role:     model.RoleUser,
		Created:  created.Format(time.RFC3339),
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	memberSince, err := repo.GetMemberSince(ctx, userID)
	if err != nil {
		t.Fatalf("GetMemberSince: %v", err)
	}
	if !memberSince.Truncate(time.Second).Equal(created.Truncate(time.Second)) {
		t.Fatalf("GetMemberSince: got %v, want %v", memberSince, created)
	}

	if _, err := repo.GetMemberSince(ctx, "missing-uuid"); err == nil {
		t.Fatal("expected 404 for missing user")
	}
}
