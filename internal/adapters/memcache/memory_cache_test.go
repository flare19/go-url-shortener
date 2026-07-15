// internal/adapters/memcache/memory_cache_test.go
package memcache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/flare19/go-url-shortener/internal/domain"
)

func TestMemoryCache_GetMiss(t *testing.T) {
	c := New()

	got, ok := c.Get(context.Background(), "nonexistent")

	if ok {
		t.Fatalf("expected miss, got hit with value %+v", got)
	}
	if got != nil {
		t.Fatalf("expected nil URL on miss, got %+v", got)
	}
}

func TestMemoryCache_SetThenGet_Hit(t *testing.T) {
	c := New()
	ctx := context.Background()

	u := &domain.URL{Code: "abc1234", LongURL: "https://example.com"}

	if err := c.Set(ctx, "abc1234", u, time.Minute); err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}

	got, ok := c.Get(ctx, "abc1234")
	if !ok {
		t.Fatal("expected hit after Set, got miss")
	}
	if got.LongURL != u.LongURL {
		t.Fatalf("got LongURL %q, want %q", got.LongURL, u.LongURL)
	}
}

func TestMemoryCache_Expiry(t *testing.T) {
	c := New()
	ctx := context.Background()

	u := &domain.URL{Code: "expiring", LongURL: "https://example.com"}

	// Negative TTL puts expiresAt in the past immediately.
	if err := c.Set(ctx, "expiring", u, -time.Second); err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}

	got, ok := c.Get(ctx, "expiring")
	if ok {
		t.Fatalf("expected miss on expired entry, got hit with value %+v", got)
	}

	// Confirm lazy eviction actually removed the entry, not just
	// masked it — check the map size directly via a second Get.
	c.mu.RLock()
	_, stillPresent := c.data["expiring"]
	c.mu.RUnlock()
	if stillPresent {
		t.Fatal("expired entry was not evicted from underlying map")
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	c := New()
	ctx := context.Background()

	u := &domain.URL{Code: "deleteme", LongURL: "https://example.com"}

	if err := c.Set(ctx, "deleteme", u, time.Minute); err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}
	if err := c.Delete(ctx, "deleteme"); err != nil {
		t.Fatalf("Delete returned unexpected error: %v", err)
	}

	_, ok := c.Get(ctx, "deleteme")
	if ok {
		t.Fatal("expected miss after Delete, got hit")
	}
}

func TestMemoryCache_DeleteNonexistent(t *testing.T) {
	c := New()

	// Deleting a key that was never set should be a no-op, not an error.
	if err := c.Delete(context.Background(), "neverexisted"); err != nil {
		t.Fatalf("Delete on nonexistent key returned unexpected error: %v", err)
	}
}

func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	c := New()
	ctx := context.Background()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			code := "code"
			u := &domain.URL{Code: code, LongURL: "https://example.com"}

			_ = c.Set(ctx, code, u, time.Minute)
			c.Get(ctx, code)
			_ = c.Delete(ctx, code)
		}(i)
	}

	wg.Wait()
}
