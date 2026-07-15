// internal/service/url_service_test.go
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flare19/go-url-shortener/internal/domain"
	"github.com/flare19/go-url-shortener/internal/ports"
)

// --- fakes ---

type fakeRepository struct {
	saveFunc      func(ctx context.Context, u *domain.URL) error
	findFunc      func(ctx context.Context, code string) (*domain.URL, error)
	incrementFunc func(ctx context.Context, deltas map[string]int64) error

	saveCalls int
	findCalls int
}

func (f *fakeRepository) Save(ctx context.Context, u *domain.URL) error {
	f.saveCalls++
	if f.saveFunc != nil {
		return f.saveFunc(ctx, u)
	}
	return nil
}

func (f *fakeRepository) FindByCode(ctx context.Context, code string) (*domain.URL, error) {
	f.findCalls++
	if f.findFunc != nil {
		return f.findFunc(ctx, code)
	}
	return nil, ports.ErrNotFound
}

func (f *fakeRepository) IncrementHitsBatch(ctx context.Context, deltas map[string]int64) error {
	if f.incrementFunc != nil {
		return f.incrementFunc(ctx, deltas)
	}
	return nil
}

type fakeEncoder struct {
	generateFunc  func(ctx context.Context) (string, error)
	generateCalls int
}

func (f *fakeEncoder) Generate(ctx context.Context) (string, error) {
	f.generateCalls++
	return f.generateFunc(ctx)
}

type fakeCache struct {
	getFunc    func(ctx context.Context, code string) (*domain.URL, bool)
	setFunc    func(ctx context.Context, code string, u *domain.URL, ttl time.Duration) error
	deleteFunc func(ctx context.Context, code string) error

	getCalls    int
	setCalls    int
	deleteCalls int
	lastSetTTL  time.Duration
}

func (f *fakeCache) Get(ctx context.Context, code string) (*domain.URL, bool) {
	f.getCalls++
	if f.getFunc != nil {
		return f.getFunc(ctx, code)
	}
	return nil, false
}

func (f *fakeCache) Set(ctx context.Context, code string, u *domain.URL, ttl time.Duration) error {
	f.setCalls++
	f.lastSetTTL = ttl
	if f.setFunc != nil {
		return f.setFunc(ctx, code, u, ttl)
	}
	return nil
}

func (f *fakeCache) Delete(ctx context.Context, code string) error {
	f.deleteCalls++
	if f.deleteFunc != nil {
		return f.deleteFunc(ctx, code)
	}
	return nil
}

// --- Create tests ---

func TestCreate_Success_WarmsCache(t *testing.T) {
	repo := &fakeRepository{}
	enc := &fakeEncoder{generateFunc: func(ctx context.Context) (string, error) {
		return "abc1234", nil
	}}
	cache := &fakeCache{}

	svc := NewURLService(repo, enc, cache)

	u, err := svc.Create(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Code != "abc1234" || u.LongURL != "https://example.com" {
		t.Fatalf("unexpected URL: %+v", u)
	}
	if repo.saveCalls != 1 {
		t.Fatalf("expected 1 Save call, got %d", repo.saveCalls)
	}
	if cache.setCalls != 1 {
		t.Fatalf("expected cache warmed once, got %d Set calls", cache.setCalls)
	}
	if cache.lastSetTTL != cacheTTL {
		t.Fatalf("expected ttl %v, got %v", cacheTTL, cache.lastSetTTL)
	}
}

func TestCreate_ValidationFailure_DoesNotRetryOrSave(t *testing.T) {
	repo := &fakeRepository{}
	enc := &fakeEncoder{generateFunc: func(ctx context.Context) (string, error) {
		return "abc1234", nil
	}}
	svc := NewURLService(repo, enc, nil)

	// Assumption: empty longURL is invalid per domain.NewURL. Adjust if
	// the real validation rule differs.
	_, err := svc.Create(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if enc.generateCalls != 1 {
		t.Fatalf("expected exactly 1 Generate call (no retry on validation failure), got %d", enc.generateCalls)
	}
	if repo.saveCalls != 0 {
		t.Fatalf("expected Save never called, got %d calls", repo.saveCalls)
	}
}

func TestCreate_RetriesOnDuplicateCode_ThenSucceeds(t *testing.T) {
	repo := &fakeRepository{}
	attempt := 0
	repo.saveFunc = func(ctx context.Context, u *domain.URL) error {
		attempt++
		if attempt < 3 {
			return ports.ErrDuplicateCode
		}
		return nil
	}
	enc := &fakeEncoder{generateFunc: func(ctx context.Context) (string, error) {
		return "abc1234", nil
	}}
	svc := NewURLService(repo, enc, nil)

	u, err := svc.Create(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u == nil {
		t.Fatal("expected non-nil URL")
	}
	if repo.saveCalls != 3 {
		t.Fatalf("expected 3 Save calls, got %d", repo.saveCalls)
	}
	if enc.generateCalls != 3 {
		t.Fatalf("expected 3 Generate calls, got %d", enc.generateCalls)
	}
}

func TestCreate_ExhaustsRetries(t *testing.T) {
	repo := &fakeRepository{saveFunc: func(ctx context.Context, u *domain.URL) error {
		return ports.ErrDuplicateCode
	}}
	enc := &fakeEncoder{generateFunc: func(ctx context.Context) (string, error) {
		return "abc1234", nil
	}}
	svc := NewURLService(repo, enc, nil)

	_, err := svc.Create(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if !errors.Is(err, ports.ErrDuplicateCode) {
		t.Fatalf("expected wrapped error to satisfy errors.Is(ErrDuplicateCode), got: %v", err)
	}
	if repo.saveCalls != maxGenerateRetries {
		t.Fatalf("expected %d Save calls, got %d", maxGenerateRetries, repo.saveCalls)
	}
	if enc.generateCalls != maxGenerateRetries {
		t.Fatalf("expected %d Generate calls, got %d", maxGenerateRetries, enc.generateCalls)
	}
}

func TestCreate_NonDuplicateRepoError_DoesNotRetry(t *testing.T) {
	dbErr := errors.New("connection refused")
	repo := &fakeRepository{saveFunc: func(ctx context.Context, u *domain.URL) error {
		return dbErr
	}}
	enc := &fakeEncoder{generateFunc: func(ctx context.Context) (string, error) {
		return "abc1234", nil
	}}
	svc := NewURLService(repo, enc, nil)

	_, err := svc.Create(context.Background(), "https://example.com")
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected underlying db error, got: %v", err)
	}
	if repo.saveCalls != 1 {
		t.Fatalf("expected exactly 1 Save call (non-duplicate errors aren't retry-worthy), got %d", repo.saveCalls)
	}
}

func TestCreate_EncoderError_ReturnsImmediately(t *testing.T) {
	encErr := errors.New("rand source exhausted")
	repo := &fakeRepository{}
	enc := &fakeEncoder{generateFunc: func(ctx context.Context) (string, error) {
		return "", encErr
	}}
	svc := NewURLService(repo, enc, nil)

	_, err := svc.Create(context.Background(), "https://example.com")
	if !errors.Is(err, encErr) {
		t.Fatalf("expected encoder error, got: %v", err)
	}
	if repo.saveCalls != 0 {
		t.Fatalf("expected Save never called, got %d", repo.saveCalls)
	}
}

func TestCreate_CacheWriteFailure_DoesNotFailRequest(t *testing.T) {
	repo := &fakeRepository{}
	enc := &fakeEncoder{generateFunc: func(ctx context.Context) (string, error) {
		return "abc1234", nil
	}}
	cache := &fakeCache{setFunc: func(ctx context.Context, code string, u *domain.URL, ttl time.Duration) error {
		return errors.New("cache unreachable")
	}}
	svc := NewURLService(repo, enc, cache)

	u, err := svc.Create(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("expected Create to succeed despite cache write failure, got: %v", err)
	}
	if u == nil {
		t.Fatal("expected non-nil URL")
	}
}

func TestCreate_NilCache_DoesNotPanic(t *testing.T) {
	repo := &fakeRepository{}
	enc := &fakeEncoder{generateFunc: func(ctx context.Context) (string, error) {
		return "abc1234", nil
	}}
	svc := NewURLService(repo, enc, nil)

	_, err := svc.Create(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error with nil cache: %v", err)
	}
}

// --- Get tests ---

func TestGet_CacheHit_SkipsRepository(t *testing.T) {
	want := &domain.URL{Code: "abc1234", LongURL: "https://example.com"}
	repo := &fakeRepository{}
	cache := &fakeCache{getFunc: func(ctx context.Context, code string) (*domain.URL, bool) {
		return want, true
	}}
	svc := NewURLService(repo, &fakeEncoder{}, cache)

	got, err := svc.Get(context.Background(), "abc1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("expected cached URL returned as-is, got %+v", got)
	}
	if repo.findCalls != 0 {
		t.Fatalf("expected repository not consulted on cache hit, got %d FindByCode calls", repo.findCalls)
	}
}

func TestGet_CacheMiss_FallsBackAndWarms(t *testing.T) {
	want := &domain.URL{Code: "abc1234", LongURL: "https://example.com"}
	repo := &fakeRepository{findFunc: func(ctx context.Context, code string) (*domain.URL, error) {
		return want, nil
	}}
	cache := &fakeCache{getFunc: func(ctx context.Context, code string) (*domain.URL, bool) {
		return nil, false
	}}
	svc := NewURLService(repo, &fakeEncoder{}, cache)

	got, err := svc.Get(context.Background(), "abc1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("expected repository URL returned, got %+v", got)
	}
	if repo.findCalls != 1 {
		t.Fatalf("expected 1 FindByCode call, got %d", repo.findCalls)
	}
	if cache.setCalls != 1 {
		t.Fatalf("expected cache warmed after repo fallback, got %d Set calls", cache.setCalls)
	}
	if cache.lastSetTTL != cacheTTL {
		t.Fatalf("expected ttl %v, got %v", cacheTTL, cache.lastSetTTL)
	}
}

func TestGet_NotFound_PropagatesErrorWithoutWarmingCache(t *testing.T) {
	repo := &fakeRepository{findFunc: func(ctx context.Context, code string) (*domain.URL, error) {
		return nil, ports.ErrNotFound
	}}
	cache := &fakeCache{getFunc: func(ctx context.Context, code string) (*domain.URL, bool) {
		return nil, false
	}}
	svc := NewURLService(repo, &fakeEncoder{}, cache)

	_, err := svc.Get(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	if cache.setCalls != 0 {
		t.Fatalf("expected cache not warmed on not-found, got %d Set calls", cache.setCalls)
	}
}

func TestGet_NilCache_GoesStraightToRepository(t *testing.T) {
	want := &domain.URL{Code: "abc1234", LongURL: "https://example.com"}
	repo := &fakeRepository{findFunc: func(ctx context.Context, code string) (*domain.URL, error) {
		return want, nil
	}}
	svc := NewURLService(repo, &fakeEncoder{}, nil)

	got, err := svc.Get(context.Background(), "abc1234")
	if err != nil {
		t.Fatalf("unexpected error with nil cache: %v", err)
	}
	if got != want {
		t.Fatalf("expected repository URL returned, got %+v", got)
	}
}
