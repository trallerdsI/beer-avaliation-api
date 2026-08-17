package moderation

import (
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewModeratorWithCache creates a Moderator with caching based on environment.
// If REDIS_URL is set, uses Redis cache; otherwise uses in-memory cache.
func NewModeratorWithCache(base Moderator) Moderator {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err == nil {
			client := redis.NewClient(opt)
			cache := NewRedisModerationCache(client, 24*time.Hour)
			return NewCachedOpenAIModerator(base, cache)
		}
		fmt.Fprintf(os.Stderr, "warning: invalid REDIS_URL %q: %v; falling back to in-memory cache\n", redisURL, err)
	}
	return NewCachedOpenAIModerator(base, NewInMemoryModerationCache())
}
