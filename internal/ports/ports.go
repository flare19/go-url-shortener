package ports

import (
	"context"
	"errors"
	"time"

	"github.com/flare19/go-url-shortener/internal/domain"
)

// Sentinel errors returned by URLRepository implementations. Adapters
// must translate their storage-specific errors (e.g. Mongo's duplicate
// key error, mongo.ErrNoDocuments) into these, so callers can branch on
// outcome without knowing which database is behind the interface.

var (
	ErrNotFound      = errors.New("ports: url not found")
	ErrDuplicateCode = errors.New("ports: dulpicate code")
)

// URLRepository defines persistence operations needed by the domain.
// Implementations live in internal/adapters (e.g. MongoURLRepository).
type URLRepository interface {
	// Save persists a new URL. Returns ErrDuplicateCode if the code
	// already exists (unique index violation) — callers (URLService)
	// should treat this as retry-worthy: generate a new code and retry,
	// not a fatal error.
	Save(ctx context.Context, u *domain.URL) error

	// FindByCode looks up a URL by its short code. Returns ErrNotFound
	// if no matching URL exists.
	FindByCode(ctx context.Context, code string) (*domain.URL, error)

	// IncrementHitsBatch applies accumulated hit-count deltas in one
	// batch call. This exists specifically to support ADR 0002's async
	// stats buffering — it should only ever be called by the stats
	// flusher, never per-request. There is deliberately no single-hit
	// increment method on this interface; if you find yourself wanting
	// one, that's a sign the redirect handler is about to bypass the
	// buffer, which defeats the point of ADR 0002.
	IncrementHitsBatch(ctx context.Context, deltas map[string]int64) error
}

// Encoder produces short codes for new URLs. The interface is
// deliberately strategy-agnostic (see ADR 0001) — it does not assume
// a counter, a random string, or any other generation scheme, so either
// approach can implement it without the contract favoring one.
//
// Collision handling is NOT this interface's responsibility. If Save
// returns ErrDuplicateCode, the caller (URLService) is responsible for
// calling Generate again — Encoder itself has no knowledge of retries.
type Encoder interface {
	Generate(ctx context.Context) (string, error)
}

// Cache is a write-through cache for URL lookups, used to absorb the
// read-your-writes gap described in ADR 0003 (a redirect immediately
// after creation landing on a stale read replica). It is deliberately
// best-effort: failures here should never fail a request.
type Cache interface {
	// Get returns the cached URL and true on a hit, or nil and false on
	// a miss or expiry. There is no error return — a cache is an
	// optimization, not a correctness path (see ADR 0003); callers
	// should treat a miss and a cache failure identically: fall back to
	// URLRepository.
	Get(ctx context.Context, code string) (*domain.URL, bool)

	// Set stores a URL in the cache for the given ttl. If it returns a
	// non-nil error, callers should log it and continue — a failed
	// cache write must never fail the request that triggered it.
	Set(ctx context.Context, code string, u *domain.URL, ttl time.Duration) error

	// Delete removes a cached entry, if present.
	Delete(ctx context.Context, code string) error
}
