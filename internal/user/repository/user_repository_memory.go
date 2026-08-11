package repository

import (
	"context"
	"sort"
	"sync"
	"time"

	"beer-review-app/internal/user/model"
	"beer-review-app/pkg/errors"
)

type InMemoryUserRepository struct {
	users       []model.User
	subscriptions []model.PushSubscription
	mutex       sync.RWMutex
	nextSubID   int
}

func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users:        []model.User{},
		subscriptions: []model.PushSubscription{},
	}
}

func (r *InMemoryUserRepository) Create(ctx context.Context, user model.User) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.users = append(r.users, user)
	return nil
}

func (r *InMemoryUserRepository) GetByID(ctx context.Context, id string) (model.User, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return model.User{}, errors.NewAppError(404, "user not found", nil)
}

func (r *InMemoryUserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return model.User{}, errors.NewAppError(404, "user not found", nil)
}

func (r *InMemoryUserRepository) GetByExternal(ctx context.Context, provider, externalSub string) (model.User, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, u := range r.users {
		if u.Provider == provider && u.ExternalSub == externalSub {
			return u, nil
		}
	}
	return model.User{}, errors.NewAppError(404, "user not found", nil)
}

func (r *InMemoryUserRepository) UpsertByExternal(ctx context.Context, user model.User) (model.User, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i, u := range r.users {
		if u.Provider == user.Provider && u.ExternalSub == user.ExternalSub {
			u.Email = user.Email
			u.Username = user.Username
			r.users[i] = u
			return u, nil
		}
	}
	r.users = append(r.users, user)
	return user, nil
}

func (r *InMemoryUserRepository) Update(ctx context.Context, id string, user model.User) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i, u := range r.users {
		if u.ID == id {
			u.Username = user.Username
			u.Email = user.Email
			r.users[i] = u
			return nil
		}
	}
	return errors.NewAppError(404, "user not found", nil)
}

func (r *InMemoryUserRepository) Delete(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i, u := range r.users {
		if u.ID == id {
			r.users = append(r.users[:i], r.users[i+1:]...)
			return nil
		}
	}
	return errors.NewAppError(404, "user not found", nil)
}

func (r *InMemoryUserRepository) List(ctx context.Context, page, pageSize int) ([]model.User, int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	sort.Slice(r.users, func(i, j int) bool {
		return r.users[i].Created > r.users[j].Created
	})
	total := len(r.users)
	start := (page - 1) * pageSize
	if start >= total || pageSize <= 0 {
		return []model.User{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return r.users[start:end], total, nil
}

func (r *InMemoryUserRepository) CreatePushSubscription(ctx context.Context, sub model.PushSubscription) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.nextSubID++
	sub.ID = string(rune('0' + r.nextSubID%10))
	if r.nextSubID > 9 {
		sub.ID = string(rune('a' + (r.nextSubID-10)%26))
	}
	sub.CreatedAt = time.Now().Format(time.RFC3339)
	sub.UpdatedAt = sub.CreatedAt
	r.subscriptions = append(r.subscriptions, sub)
	return nil
}

func (r *InMemoryUserRepository) ListPushSubscriptions(ctx context.Context, userID string) ([]model.PushSubscription, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	var out []model.PushSubscription
	for _, s := range r.subscriptions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *InMemoryUserRepository) DeletePushSubscription(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i, s := range r.subscriptions {
		if s.ID == id {
			r.subscriptions = append(r.subscriptions[:i], r.subscriptions[i+1:]...)
			return nil
		}
	}
	return errors.NewAppError(404, "push subscription not found", nil)
}

func (r *InMemoryUserRepository) DeletePushSubscriptionByEndpoint(ctx context.Context, userID, endpoint string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i, s := range r.subscriptions {
		if s.UserID == userID && s.Endpoint == endpoint {
			r.subscriptions = append(r.subscriptions[:i], r.subscriptions[i+1:]...)
			return nil
		}
	}
	return errors.NewAppError(404, "push subscription not found", nil)
}

func (r *InMemoryUserRepository) GetMemberSince(ctx context.Context, userID string) (time.Time, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, u := range r.users {
		if u.ID == userID {
			t, _ := time.Parse(time.RFC3339, u.Created)
			return t, nil
		}
	}
	return time.Time{}, errors.NewAppError(404, "user not found", nil)
}
