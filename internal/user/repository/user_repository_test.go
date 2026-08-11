package repository

import (
	"context"
	"testing"
	"time"

	"beer-review-app/internal/user/model"
)

func TestInMemoryUserRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryUserRepository()

	user := model.User{ID: "u1", Username: "alice", Email: "alice@example.com", Role: model.RoleUser, Created: time.Now().Format(time.RFC3339)}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Username != "alice" {
		t.Fatalf("Username: got %q, want alice", got.Username)
	}

	got, err = repo.GetByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.ID != "u1" {
		t.Fatalf("GetByEmail ID: got %q, want u1", got.ID)
	}

	u := model.User{ID: "u2", Username: "bob", Email: "bob@example.com", Provider: "google", ExternalSub: "sub-123", Role: model.RoleUser, Created: time.Now().Format(time.RFC3339)}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create external: %v", err)
	}
	got, err = repo.GetByExternal(ctx, "google", "sub-123")
	if err != nil {
		t.Fatalf("GetByExternal: %v", err)
	}
	if got.Username != "bob" {
		t.Fatalf("GetByExternal Username: got %q, want bob", got.Username)
	}

	updated, err := repo.UpsertByExternal(ctx, model.User{Username: "bob2", Email: "bob2@example.com", Provider: "google", ExternalSub: "sub-456", Role: model.RoleUser, Created: time.Now().Format(time.RFC3339)})
	if err != nil {
		t.Fatalf("UpsertByExternal new: %v", err)
	}
	if updated.Username != "bob2" {
		t.Fatalf("UpsertByExternal new Username: got %q, want bob2", updated.Username)
	}
	updated, err = repo.UpsertByExternal(ctx, model.User{Username: "bob3", Email: "bob3@example.com", Provider: "google", ExternalSub: "sub-456", Role: model.RoleUser, Created: time.Now().Format(time.RFC3339)})
	if err != nil {
		t.Fatalf("UpsertByExternal update: %v", err)
	}
	if updated.Username != "bob3" {
		t.Fatalf("UpsertByExternal update Username: got %q, want bob3", updated.Username)
	}

	if err := repo.Update(ctx, "u1", model.User{Username: "alice2", Email: "alice@example.com", Role: model.RoleUser, Created: time.Now().Format(time.RFC3339)}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.GetByID(ctx, "u1")
	if got.Username != "alice2" {
		t.Fatalf("Update Username: got %q, want alice2", got.Username)
	}

	users, total, err := repo.List(ctx, 1, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Fatalf("List total: got %d, want 3", total)
	}
	if len(users) != 3 {
		t.Fatalf("List len: got %d, want 3", len(users))
	}

	if err := repo.Delete(ctx, "u1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, "u1"); err == nil {
		t.Fatal("expected 404 after delete")
	}
}

func TestInMemoryPushSubscriptions(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryUserRepository()
	user := model.User{ID: "u1", Username: "a", Email: "a@e.com", Role: model.RoleUser, Created: time.Now().Format(time.RFC3339)}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	sub := model.PushSubscription{UserID: "u1", Endpoint: "https://push.example.com", P256DH: "key1", Auth: "auth1"}
	if err := repo.CreatePushSubscription(ctx, sub); err != nil {
		t.Fatalf("CreatePushSubscription: %v", err)
	}

	subs, err := repo.ListPushSubscriptions(ctx, "u1")
	if err != nil {
		t.Fatalf("ListPushSubscriptions: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("ListPushSubscriptions len: got %d, want 1", len(subs))
	}

	if err := repo.DeletePushSubscriptionByEndpoint(ctx, "u1", "https://push.example.com"); err != nil {
		t.Fatalf("DeletePushSubscriptionByEndpoint: %v", err)
	}
	subs, _ = repo.ListPushSubscriptions(ctx, "u1")
	if len(subs) != 0 {
		t.Fatalf("ListPushSubscriptions after delete: got %d, want 0", len(subs))
	}

	if err := repo.DeletePushSubscription(ctx, "missing"); err == nil {
		t.Fatal("expected 404 for missing push subscription")
	}
}

func TestInMemoryGetMemberSince(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryUserRepository()
	created := time.Now().AddDate(0, 0, -10)
	user := model.User{ID: "u1", Username: "a", Email: "a@e.com", Role: model.RoleUser, Created: created.Format(time.RFC3339)}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	memberSince, err := repo.GetMemberSince(ctx, "u1")
	if err != nil {
		t.Fatalf("GetMemberSince: %v", err)
	}
	if !memberSince.Truncate(time.Second).Equal(created.Truncate(time.Second)) {
		t.Fatalf("GetMemberSince: got %v, want %v", memberSince, created)
	}

	if _, err := repo.GetMemberSince(ctx, "missing"); err == nil {
		t.Fatal("expected 404 for missing user")
	}
}
