package events

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Event types emitted by the beer domain.
	TypeBeerCreated    = "beer.created"
	TypeBeerUpdated    = "beer.updated"
	TypeBeerDeleted    = "beer.deleted"
	TypeCommentAdded   = "comment.added"
	TypeCommentDeleted = "comment.deleted"
	TypeCommentLiked   = "comment.liked"
	TypeMediaAdded     = "beer.media.added"
)

const maxEventsPerBeer = 1000

// Event represents a domain event emitted when beer data changes.
type Event struct {
	Type      string         `json:"type"`
	ID        string         `json:"id"`
	BeerID    string         `json:"beerId"`
	Data      map[string]any `json:"data"`
	Timestamp time.Time      `json:"timestamp"`
}

// Publisher emits domain events to an event store.
type Publisher interface {
	Publish(ctx context.Context, beerID string, ev Event) error
}

// Store reads domain events for a given beer, optionally after a timestamp.
type Store interface {
	ListSince(ctx context.Context, beerID string, since time.Time) ([]Event, error)
	LatestEvent(ctx context.Context, beerID string) (score float64, member string, err error)
}

// EventStore combines Publisher and Store into a single interface.
type EventStore interface {
	Publisher
	Store
}

// SanitizeETag creates a safe Weak ETag from a Redis member string.
// It hashes the member to avoid HTTP header injection from quotes/newlines.
func SanitizeETag(member string) string {
	if member == "" {
		return `W/"0"`
	}
	hash := sha256.Sum256([]byte(member))
	return fmt.Sprintf(`W/"%x"`, hash[:8])
}

// redisStore implements Publisher and Store using a Redis sorted set per beer.
type redisStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewRedisStore creates a Publisher/Store backed by Redis.
// ttl is the retention period for events per beer key.
func NewRedisStore(client *redis.Client, prefix string, ttl time.Duration) EventStore {
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

	pipe := s.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: payload})
	pipe.ZRemRangeByRank(ctx, key, 0, -maxEventsPerBeer-1)
	if s.ttl > 0 {
		pipe.Expire(ctx, key, s.ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
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
