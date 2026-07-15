// internal/adapters/memcache/memory_cache.go
package memcache

import (
	"context"
	"sync"
	"time"

	"github.com/flare19/go-url-shortener/internal/domain"
)

// MemoryCache is an in-memory implementation of ports.Cache.
// It is the v1 cache per ADR 0003 — safe for concurrent access,
// entries expire lazily (checked on Get, no background sweeper).
type MemoryCache struct {
	mu   sync.RWMutex
	data map[string]cacheEntry
}

type cacheEntry struct {
	url       *domain.URL
	expiresAt time.Time
}

// New returns an empty, ready-to-use MemoryCache.
func New() *MemoryCache {
	return &MemoryCache{
		data: make(map[string]cacheEntry),
	}
}

func (c *MemoryCache) Get(ctx context.Context, code string) (*domain.URL, bool) {
	c.mu.RLock()
	entry, ok := c.data[code]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		// Expired — lazily evict and report as a miss.
		c.mu.Lock()
		delete(c.data, code)
		c.mu.Unlock()
		return nil, false
	}

	return entry.url, true
}

func (c *MemoryCache) Set(ctx context.Context, code string, u *domain.URL, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[code] = cacheEntry{
		url:       u,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (c *MemoryCache) Delete(ctx context.Context, code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, code)
	return nil
}
