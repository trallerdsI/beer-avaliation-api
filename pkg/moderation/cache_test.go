package moderation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryModerationCache_GetSet(t *testing.T) {
	cache := NewInMemoryModerationCache()
	ctx := context.Background()

	_, found, err := cache.Get(ctx, "key1")
	require.NoError(t, err)
	assert.False(t, found)

	require.NoError(t, cache.Set(ctx, "key1", true))
	val, found, err := cache.Get(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.True(t, val)

	require.NoError(t, cache.Set(ctx, "key1", false))
	val, found, err = cache.Get(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.False(t, val)
}

func TestInMemoryModerationCache_ConcurrentAccess(t *testing.T) {
	cache := NewInMemoryModerationCache()
	ctx := context.Background()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "concurrent-key"
			for j := 0; j < 100; j++ {
				_ = cache.Set(ctx, key, j%2 == 0)
				_, _, _ = cache.Get(ctx, key)
			}
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRedisModerationCache_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Redis integration test in short mode")
	}

	// This test requires a running Redis instance.
	// Set REDIS_URL environment variable to run this test.
	// We verify the constructor works and returns a valid struct.
	// Actual Redis integration should be tested with testcontainers or miniredis.
	cache := NewRedisModerationCache(nil, 24*time.Hour)
	require.NotNil(t, cache)
	assert.Equal(t, 24*time.Hour, cache.ttl)
}

func TestCachedOpenAIModerator_CachesResult(t *testing.T) {
	callCount := 0
	mockModerator := &mockModerator{
		allowed: true,
		onCall:  func() { callCount++ },
	}

	cache := NewInMemoryModerationCache()
	cachedMod := NewCachedOpenAIModerator(mockModerator, cache)
	ctx := context.Background()

	allowed, err := cachedMod.IsContentAllowed(ctx, "test text")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, callCount)

	allowed, err = cachedMod.IsContentAllowed(ctx, "test text")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, callCount, "should use cache on second call")
}

func TestCachedOpenAIModerator_FailOpenOnCacheError(t *testing.T) {
	failingCache := &failingModerationCache{}
	mockModerator := &mockModerator{allowed: true}

	cachedMod := NewCachedOpenAIModerator(mockModerator, failingCache)
	ctx := context.Background()

	allowed, err := cachedMod.IsContentAllowed(ctx, "test text")
	require.NoError(t, err)
	assert.True(t, allowed, "should fail open when cache fails")
}

type mockModerator struct {
	allowed bool
	onCall  func()
}

func (m *mockModerator) IsContentAllowed(_ context.Context, _ string) (bool, error) {
	if m.onCall != nil {
		m.onCall()
	}
	return m.allowed, nil
}

type failingModerationCache struct{}

func (c *failingModerationCache) Get(_ context.Context, _ string) (bool, bool, error) {
	return false, false, assert.AnError
}

func (c *failingModerationCache) Set(_ context.Context, _ string, _ bool) error {
	return assert.AnError
}
