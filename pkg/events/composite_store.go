package events

import (
	"context"
	"time"
)

// CompositeStore tries a primary Store first, falling back to a secondary
// Store when the primary returns no events or an error. This prevents data
// loss when Redis keys expire (TTL) while ensuring PostgreSQL remains the
// source of truth.
type CompositeStore struct {
	primary  Publisher
	fallback Store
}

// NewCompositeStore creates a CompositeStore with the given stores.
// primary should be the fast cache (Redis); fallback should be durable (Postgres).
func NewCompositeStore(primary Publisher, fallback Store) *CompositeStore {
	return &CompositeStore{primary: primary, fallback: fallback}
}

// ListSince returns events since a timestamp, trying primary first.
func (c *CompositeStore) ListSince(ctx context.Context, beerID string, since time.Time) ([]Event, error) {
	if s, ok := c.primary.(Store); ok {
		events, err := s.ListSince(ctx, beerID, since)
		if err == nil && len(events) > 0 {
			return events, nil
		}
	}
	if c.fallback != nil {
		return c.fallback.ListSince(ctx, beerID, since)
	}
	return nil, nil
}

// LatestEvent returns the most recent event metadata, trying primary first.
func (c *CompositeStore) LatestEvent(ctx context.Context, beerID string) (float64, string, error) {
	if s, ok := c.primary.(Store); ok {
		score, member, err := s.LatestEvent(ctx, beerID)
		if err == nil && member != "" {
			return score, member, err
		}
	}
	if c.fallback != nil {
		return c.fallback.LatestEvent(ctx, beerID)
	}
	return 0, "", nil
}

// Publish publishes to the primary store only (writes go to Redis cache).
func (c *CompositeStore) Publish(ctx context.Context, beerID string, ev Event) error {
	if c.primary != nil {
		return c.primary.Publish(ctx, beerID, ev)
	}
	return nil
}
