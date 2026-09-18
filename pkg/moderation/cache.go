package moderation

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/redis/go-redis/v9"

	"beer-review-app/pkg/metrics"
)

// ModerationCache defines the interface for caching moderation results.
type ModerationCache interface {
	Get(ctx context.Context, key string) (bool, bool, error)
	Set(ctx context.Context, key string, value bool) error
}

// InMemoryModerationCache implements ModerationCache using an LRU cache with TTL.
// Bounded size (10k entries) and 24h TTL prevent unbounded memory growth.
type InMemoryModerationCache struct {
	lru *expirable.LRU[string, bool]
}

func NewInMemoryModerationCache() *InMemoryModerationCache {
	return &InMemoryModerationCache{
		lru: expirable.NewLRU[string, bool](10000, nil, 24*time.Hour),
	}
}

func (c *InMemoryModerationCache) Get(_ context.Context, key string) (bool, bool, error) {
	val, ok := c.lru.Get(key)
	if ok {
		metrics.ModerationCacheHitsTotal.WithLabelValues("memory").Inc()
	}
	return val, ok, nil
}

func (c *InMemoryModerationCache) Set(_ context.Context, key string, value bool) error {
	c.lru.Add(key, value)
	return nil
}

// RedisModerationCache implements ModerationCache using Redis.
type RedisModerationCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisModerationCache(client *redis.Client, ttl time.Duration) *RedisModerationCache {
	return &RedisModerationCache{
		client: client,
		ttl:    ttl,
	}
}

func (c *RedisModerationCache) Get(ctx context.Context, key string) (bool, bool, error) {
	cmd := c.client.Get(ctx, key)
	if cmd.Err() == redis.Nil {
		return false, false, nil
	}
	if cmd.Err() != nil {
		return false, false, fmt.Errorf("redis get: %w", cmd.Err())
	}
	val, err := cmd.Bool()
	if err != nil {
		return false, false, fmt.Errorf("redis bool conversion: %w", err)
	}
	metrics.ModerationCacheHitsTotal.WithLabelValues("redis").Inc()
	return val, true, nil
}

func (c *RedisModerationCache) Set(ctx context.Context, key string, value bool) error {
	if err := c.client.Set(ctx, key, value, c.ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}
