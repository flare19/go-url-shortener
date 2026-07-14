package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/flare19/go-url-shortener/internal/domain"
	"github.com/flare19/go-url-shortener/internal/ports"
)

// maxGenerateRetries bounds how many times URLService will retry code
// generation on a collision before giving up. Prevents an infinite loop
// if the encoder is somehow producing only taken codes.
const maxGenerateRetries = 5

// cacheTTL is how long a newly created URL stays in the cache, per
// ADR 0003 — long enough to absorb the read-your-writes window right
// after creation, short enough that the cache never drifts far from
// Mongo as the source of truth.
const cacheTTL = 60 * time.Second

// URLService orchestrates URL creation and lookup. It depends only on
// the ports interfaces, not on any concrete adapter — this is the
// composition point: URLService is built out of interchangeable pieces
// injected at construction time, not a fixed base type.
type URLService struct {
	repo    ports.URLRepository
	encoder ports.Encoder
	cache   ports.Cache
}

// NewURLService constructs a URLService from its dependencies. cache may
// be nil if caching isn't wired up yet — Create and Get both check for
// this before using it.
func NewURLService(repo ports.URLRepository, encoder ports.Encoder, cache ports.Cache) *URLService {
	return &URLService{
		repo:    repo,
		encoder: encoder,
		cache:   cache,
	}
}

// Create validates and persists a new short URL for longURL, generating
// a code via the configured Encoder. On a code collision
// (ports.ErrDuplicateCode), it retries generation up to
// maxGenerateRetries times before giving up.
func (s *URLService) Create(ctx context.Context, longURL string) (*domain.URL, error) {
	var lastErr error

	for attempt := 0; attempt < maxGenerateRetries; attempt++ {
		code, err := s.encoder.Generate(ctx)
		if err != nil {
			return nil, err
		}

		u, err := domain.NewURL(code, longURL)
		if err != nil {
			// Validation failure isn't retry-worthy — the long URL
			// itself is invalid regardless of what code we generated.
			return nil, err
		}

		err = s.repo.Save(ctx, u)
		if err == nil {
			s.warmCache(ctx, u)
			return u, nil
		}

		if errors.Is(err, ports.ErrDuplicateCode) {
			lastErr = err
			continue
		}

		// Any other error (DB down, etc.) is not retry-worthy.
		return nil, err
	}

	return nil, errors.Join(errors.New("service: exhausted retries generating unique code"), lastErr)
}

// Get looks up a URL by code, checking the cache before falling back to
// the repository. A cache miss or cache failure both fall through to the
// repository identically — the cache is best-effort (ADR 0003).
func (s *URLService) Get(ctx context.Context, code string) (*domain.URL, error) {
	if s.cache != nil {
		if u, ok := s.cache.Get(ctx, code); ok {
			return u, nil
		}
	}

	u, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	s.warmCache(ctx, u)
	return u, nil
}

// warmCache populates the cache and logs, but never fails, on a cache
// write error — per ADR 0003, cache writes are best-effort.
func (s *URLService) warmCache(ctx context.Context, u *domain.URL) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Set(ctx, u.Code, u, cacheTTL); err != nil {
		slog.Warn("cache set failed", "code", u.Code, "error", err)
	}
}
