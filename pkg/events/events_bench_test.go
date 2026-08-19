package events

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func BenchmarkRedisStore_Publish(b *testing.B) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	store := &redisStore{client: client, prefix: "bench", ttl: 0}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Publish(context.Background(), "beer-1", Event{
			Type:      "beer.created",
			ID:        "evt-1",
			BeerID:    "beer-1",
			Data:      map[string]any{"name": "IPA"},
			Timestamp: time.Now(),
		})
	}
}

func BenchmarkRedisStore_ListSince(b *testing.B) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	store := &redisStore{client: client, prefix: "bench", ttl: 0}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.ListSince(context.Background(), "beer-1", time.Now())
	}
}
