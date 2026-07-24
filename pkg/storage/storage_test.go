package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewSupabaseStorageFromEnvMissingVars(t *testing.T) {
	origURL := os.Getenv("SUPABASE_URL")
	origBucket := os.Getenv("SUPABASE_STORAGE_BUCKET")
	origKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	defer func() {
		os.Setenv("SUPABASE_URL", origURL)
		os.Setenv("SUPABASE_STORAGE_BUCKET", origBucket)
		os.Setenv("SUPABASE_SERVICE_ROLE_KEY", origKey)
	}()

	os.Unsetenv("SUPABASE_URL")
	os.Unsetenv("SUPABASE_STORAGE_BUCKET")
	os.Unsetenv("SUPABASE_SERVICE_ROLE_KEY")

	_, err := NewSupabaseStorageFromEnv()
	if err == nil {
		t.Fatal("expected error when env vars are missing")
	}
}

func TestNewSupabaseStorageFromEnvOK(t *testing.T) {
	origURL := os.Getenv("SUPABASE_URL")
	origBucket := os.Getenv("SUPABASE_STORAGE_BUCKET")
	origKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	defer func() {
		os.Setenv("SUPABASE_URL", origURL)
		os.Setenv("SUPABASE_STORAGE_BUCKET", origBucket)
		os.Setenv("SUPABASE_SERVICE_ROLE_KEY", origKey)
	}()

	os.Setenv("SUPABASE_URL", "https://test.supabase.co")
	os.Setenv("SUPABASE_STORAGE_BUCKET", "media")
	os.Setenv("SUPABASE_SERVICE_ROLE_KEY", "secret")

	s, err := NewSupabaseStorageFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil storage")
	}
}

func TestUploadSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected auth: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "image/png" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	s := &SupabaseStorage{
		baseURL:    ts.URL,
		bucket:     "media",
		serviceKey: "secret",
		client:     http.DefaultClient,
	}

	url, err := s.Upload(context.Background(), "obj", "image/png", []byte("data"))
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if url != ts.URL+"/storage/v1/object/public/media/obj" {
		t.Fatalf("unexpected url: %s", url)
	}
}

func TestUploadServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	s := &SupabaseStorage{
		baseURL:    ts.URL,
		bucket:     "media",
		serviceKey: "secret",
		client:     http.DefaultClient,
	}

	_, err := s.Upload(context.Background(), "obj", "image/png", []byte("data"))
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestPublicURL(t *testing.T) {
	s := &SupabaseStorage{baseURL: "https://test.supabase.co", bucket: "media"}
	got := s.publicURL("beer.jpg")
	want := "https://test.supabase.co/storage/v1/object/public/media/beer.jpg"
	if got != want {
		t.Fatalf("publicURL: got %s, want %s", got, want)
	}
}
