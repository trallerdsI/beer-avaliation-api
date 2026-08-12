package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"beer-review-app/internal/user/model"
)

type fuzzUserUsecase struct{}

func (f *fuzzUserUsecase) Register(ctx context.Context, u model.User) error {
	return nil
}
func (f *fuzzUserUsecase) Login(ctx context.Context, email, password string) (string, error) {
	return "token", nil
}
func (f *fuzzUserUsecase) OAuthLogin(ctx context.Context, provider, idToken string) (string, error) {
	return "token", nil
}
func (f *fuzzUserUsecase) GetProfile(ctx context.Context, id string) (model.User, error) {
	return model.User{}, nil
}
func (f *fuzzUserUsecase) UpdateProfile(ctx context.Context, id string, u model.User) error {
	return nil
}
func (f *fuzzUserUsecase) DeleteAccount(ctx context.Context, id string) error {
	return nil
}
func (f *fuzzUserUsecase) SeedAdmin(ctx context.Context) error {
	return nil
}
func (f *fuzzUserUsecase) SubscribePush(ctx context.Context, userID string, sub model.PushSubscription) error {
	return nil
}
func (f *fuzzUserUsecase) UnsubscribePush(ctx context.Context, userID, endpoint string) error {
	return nil
}
func (f *fuzzUserUsecase) ListPushSubscriptions(ctx context.Context, userID string) ([]model.PushSubscription, error) {
	return nil, nil
}

func FuzzRegisterHandler(f *testing.F) {
	f.Add(`{"name":"John","email":"john@example.com","password":"secret123"}`)
	f.Add(`{}`)
	f.Add(`{"name":"","email":"invalid","password":""}`)
	f.Add(`{"email":"not-an-email","password":"short"}`)

	controller := NewUserController(&fuzzUserUsecase{}, nil)

	f.Fuzz(func(t *testing.T, data string) {
		body := bytes.NewReader([]byte(data))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		controller.Register(w, req)
	})
}

func FuzzLoginHandler(f *testing.F) {
	f.Add(`{"email":"john@example.com","password":"secret123"}`)
	f.Add(`{}`)
	f.Add(`{"email":"invalid","password":""}`)
	f.Add(`{"email":"john@example.com"}`)

	controller := NewUserController(&fuzzUserUsecase{}, nil)

	f.Fuzz(func(t *testing.T, data string) {
		body := bytes.NewReader([]byte(data))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		controller.Login(w, req)
	})
}

func FuzzOAuthHandler(f *testing.F) {
	f.Add(`{"provider":"google","id_token":"valid.token.here"}`)
	f.Add(`{}`)
	f.Add(`{"provider":"","id_token":""}`)
	f.Add(`{"provider":"google"}`)

	controller := NewUserController(&fuzzUserUsecase{}, nil)

	f.Fuzz(func(t *testing.T, data string) {
		body := bytes.NewReader([]byte(data))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/oauth/google", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		controller.OAuth(w, req)
	})
}

func FuzzUpdateProfileHandler(f *testing.F) {
	f.Add(`{"name":"John Updated","email":"john@example.com"}`)
	f.Add(`{}`)
	f.Add(`{"name":"","email":"invalid"}`)

	controller := NewUserController(&fuzzUserUsecase{}, nil)

	f.Fuzz(func(t *testing.T, data string) {
		body := bytes.NewReader([]byte(data))
		req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-1", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		controller.UpdateProfile(w, req)
	})
}

func FuzzSubscribePushHandler(f *testing.F) {
	f.Add(`{"endpoint":"https://push.example.com","p256dh":"key","auth":"secret"}`)
	f.Add(`{}`)
	f.Add(`{"endpoint":"","p256dh":"","auth":""}`)

	controller := NewUserController(&fuzzUserUsecase{}, nil)

	f.Fuzz(func(t *testing.T, data string) {
		body := bytes.NewReader([]byte(data))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users/user-1/push/subscribe", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		controller.SubscribePush(w, req)
	})
}

func FuzzUnsubscribePushHandler(f *testing.F) {
	f.Add(`{"endpoint":"https://push.example.com"}`)
	f.Add(`{}`)
	f.Add(`{"endpoint":""}`)

	controller := NewUserController(&fuzzUserUsecase{}, nil)

	f.Fuzz(func(t *testing.T, data string) {
		body := bytes.NewReader([]byte(data))
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/user-1/push/subscribe", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		controller.UnsubscribePush(w, req)
	})
}

func FuzzDecodeUserJSON(f *testing.F) {
	f.Add(`{"name":"John","email":"john@example.com","password":"secret123","role":"user"}`)
	f.Add(`{"name":"","email":"invalid","password":""}`)
	f.Add(`{"email":"john@example.com"}`)

	f.Fuzz(func(t *testing.T, data string) {
		var u model.User
		if err := json.Unmarshal([]byte(data), &u); err != nil {
			return
		}
		_, _ = json.Marshal(u)
	})
}
