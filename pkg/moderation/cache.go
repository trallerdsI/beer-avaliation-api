package moderation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ModerationCache defines the interface for caching moderation results.
type ModerationCache interface {
	Get(ctx context.Context, key string) (bool, bool, error)
	Set(ctx context.Context, key string, value bool) error
}

// InMemoryModerationCache implements ModerationCache using a simple map.
type InMemoryModerationCache struct {
	mu    sync.RWMutex
	cache map[string]bool
}

func NewInMemoryModerationCache() *InMemoryModerationCache {
	return &InMemoryModerationCache{
		cache: make(map[string]bool),
	}
}

func (c *InMemoryModerationCache) Get(_ context.Context, key string) (bool, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.cache[key]
	return val, ok, nil
}

func (c *InMemoryModerationCache) Set(_ context.Context, key string, value bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = value
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
	return val, true, nil
}

func (c *RedisModerationCache) Set(ctx context.Context, key string, value bool) error {
	if err := c.client.Set(ctx, key, value, c.ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}
