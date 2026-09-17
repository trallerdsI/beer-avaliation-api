package events

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeEventStore struct {
	publishErr error
}

func (s *fakeEventStore) Publish(context.Context, string, Event) error {
	return s.publishErr
}

func (s *fakeEventStore) ListSince(context.Context, string, time.Time) ([]Event, error) {
	return nil, nil
}

func (s *fakeEventStore) LatestEvent(context.Context, string) (float64, string, error) {
	return 0, "", nil
}

func TestCompositeStorePublishIgnoresCacheFailure(t *testing.T) {
	durable := &fakeEventStore{}
	cache := &fakeEventStore{publishErr: errors.New("redis unavailable")}

	if err := NewCompositeStore(cache, durable).Publish(context.Background(), "beer-1", Event{}); err != nil {
		t.Fatalf("expected durable publish to succeed despite cache failure, got %v", err)
	}
}

func TestCompositeStorePublishReturnsDurableFailure(t *testing.T) {
	durable := &fakeEventStore{publishErr: errors.New("postgres unavailable")}
	cache := &fakeEventStore{}

	if err := NewCompositeStore(cache, durable).Publish(context.Background(), "beer-1", Event{}); err == nil {
		t.Fatal("expected durable publish failure")
	}
}
