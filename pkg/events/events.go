package events

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Event types emitted by the beer domain.
	TypeBeerCreated     = "beer.created"
	TypeBeerUpdated     = "beer.updated"
	TypeBeerDeleted     = "beer.deleted"
	TypeCommentAdded    = "comment.added"
	TypeCommentDeleted  = "comment.deleted"
	TypeCommentLiked    = "comment.liked"
	TypeMediaAdded      = "beer.media.added"
)

// Event represents a domain event emitted when beer data changes.
type Event struct {
	Type      string                 `json:"type"`
	ID        string                 `json:"id"`
	BeerID    string                 `json:"beerId"`
	Data      map[string]any         `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// Publisher emits domain events to an event store.
type Publisher interface {
	Publish(ctx context.Context, beerID string, ev Event) error
}

// Store reads domain events for a given beer, optionally after a timestamp.
type Store interface {
	ListSince(ctx context.Context, beerID string, since time.Time) ([]Event, error)
	LatestEvent(ctx context.Context, beerID string) (score float64, member string, err error)
	// ListSinceWithLatest returns events and the latest event metadata in a
	// single Redis pipeline round-trip. Use this to avoid double-fetch on
	// cache misses.
	ListSinceWithLatest(ctx context.Context, beerID string, since time.Time) (events []Event, latestMember string, err error)
}

// redisStore implements Publisher and Store using a Redis sorted set per beer.
type redisStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewRedisStore creates a Publisher/Store backed by Redis.
// ttl is the retention period for events per beer key.
func NewRedisStore(client *redis.Client, prefix string, ttl time.Duration) Publisher {
	return &redisStore{client: client, prefix: prefix, ttl: ttl}
}

func (s *redisStore) key(beerID string) string {
	return s.prefix + ":beer:" + beerID
}

func (s *redisStore) Publish(ctx context.Context, beerID string, ev Event) error {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	score := float64(ev.Timestamp.UnixNano())
	key := s.key(beerID)
	if err := s.client.ZAdd(ctx, key, redis.Z{Score: score, Member: payload}).Err(); err != nil {
		return err
	}
	if s.ttl > 0 {
		_ = s.client.Expire(ctx, key, s.ttl)
	}
	return nil
}

func (s *redisStore) ListSince(ctx context.Context, beerID string, since time.Time) ([]Event, error) {
	key := s.key(beerID)
	min := "-inf"
	if !since.IsZero() {
		min = strconv.FormatInt(since.UnixNano(), 10)
	}
	cmd := s.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: min,
		Max: "+inf",
	})
	if cmd.Err() != nil {
		return nil, cmd.Err()
	}
	events := make([]Event, 0, len(cmd.Val()))
	for _, raw := range cmd.Val() {
		var ev Event
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

// LatestEvent returns the score and raw member of the most recent event
// without deserializing the JSON payload. O(1) for ETag calculation.
func (s *redisStore) LatestEvent(ctx context.Context, beerID string) (float64, string, error) {
	key := s.key(beerID)
	cmd := s.client.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Offset: 0,
		Count:  1,
	})
	if cmd.Err() != nil {
		return 0, "", cmd.Err()
	}
	if len(cmd.Val()) == 0 {
		return 0, "", nil
	}
	member := cmd.Val()[0]
	scores, err := s.client.ZScore(ctx, key, member).Result()
	if err != nil {
		return 0, "", err
	}
	return scores, member, nil
}

// ListSinceWithLatest returns events since a timestamp and the raw member of
// the most recent event in a single Redis pipeline round-trip. This avoids
// the double-fetch penalty on cache misses.
func (s *redisStore) ListSinceWithLatest(ctx context.Context, beerID string, since time.Time) ([]Event, string, error) {
	key := s.key(beerID)
	min := "-inf"
	if !since.IsZero() {
		min = strconv.FormatInt(since.UnixNano(), 10)
	}

	pipe := s.client.Pipeline()
	latestCmd := pipe.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Offset: 0,
		Count:  1,
	})
	listCmd := pipe.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: min,
		Max: "+inf",
	})
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, "", err
	}
	if latestCmd.Err() != nil {
		return nil, "", latestCmd.Err()
	}
	if listCmd.Err() != nil {
		return nil, "", listCmd.Err()
	}

	var latestMember string
	if len(latestCmd.Val()) > 0 {
		latestMember = latestCmd.Val()[0]
	}

	events := make([]Event, 0, len(listCmd.Val()))
	for _, raw := range listCmd.Val() {
		var ev Event
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, latestMember, nil
}
