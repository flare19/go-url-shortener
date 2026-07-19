// cmd/redirector/main_test.go
package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/flare19/go-url-shortener/internal/adapters/stats"
	"github.com/flare19/go-url-shortener/internal/domain"
	"github.com/flare19/go-url-shortener/internal/ports"
	"github.com/flare19/go-url-shortener/internal/service"
)

// --- local stubs, scoped to this package's tests only ---

type stubRepository struct {
	url    *domain.URL
	err    error
	called bool
}

func (s *stubRepository) Save(ctx context.Context, u *domain.URL) error {
	return nil
}

func (s *stubRepository) FindByCode(ctx context.Context, code string) (*domain.URL, error) {
	s.called = true
	return s.url, s.err
}

func (s *stubRepository) IncrementHitsBatch(ctx context.Context, deltas map[string]int64) error {
	return nil
}

type setCall struct {
	code string
	url  *domain.URL
	ttl  time.Duration
}

type stubCache struct {
	getURL   *domain.URL
	getHit   bool
	setCalls []setCall
	setErr   error
}

func (c *stubCache) Get(ctx context.Context, code string) (*domain.URL, bool) {
	return c.getURL, c.getHit
}

func (c *stubCache) Set(ctx context.Context, code string, u *domain.URL, ttl time.Duration) error {
	c.setCalls = append(c.setCalls, setCall{code: code, url: u, ttl: ttl})
	return c.setErr
}

func (c *stubCache) Delete(ctx context.Context, code string) error {
	return nil
}

// --- helpers ---

func newTestRequest(code string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/"+code, nil)
	return mux.SetURLVars(req, map[string]string{"code": code})
}

// add near the top, with the other helpers
func newTestStatsBuffer() *stats.StatsBuffer {
	return stats.NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error { return nil },
		time.Minute, // interval is irrelevant, Start() is never called in these tests
	)
}

// --- tests ---

func TestRedirectHandler_CacheHit(t *testing.T) {
	cachedURL := &domain.URL{Code: "abc1234", LongURL: "https://example.com/cached"}
	repo := &stubRepository{} // should never be called on a cache hit
	cache := &stubCache{getURL: cachedURL, getHit: true}

	svc := service.NewURLService(repo, nil, cache)
	handler := redirectHandler(svc, newTestStatsBuffer())

	req := newTestRequest("abc1234")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); loc != cachedURL.LongURL {
		t.Errorf("Location = %q, want %q", loc, cachedURL.LongURL)
	}
	if repo.called {
		t.Error("repo.FindByCode was called on a cache hit, expected cache-only path")
	}
}

func TestRedirectHandler_CacheMissFallbackToRepo(t *testing.T) {
	repoURL := &domain.URL{Code: "abc1234", LongURL: "https://example.com/from-repo"}
	repo := &stubRepository{url: repoURL, err: nil}
	cache := &stubCache{getHit: false}

	svc := service.NewURLService(repo, nil, cache)
	handler := redirectHandler(svc, newTestStatsBuffer())

	req := newTestRequest("abc1234")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); loc != repoURL.LongURL {
		t.Errorf("Location = %q, want %q", loc, repoURL.LongURL)
	}
	if !repo.called {
		t.Error("repo.FindByCode was not called on a cache miss")
	}
	if len(cache.setCalls) != 1 {
		t.Fatalf("expected 1 cache warm call, got %d", len(cache.setCalls))
	}
	if cache.setCalls[0].code != "abc1234" {
		t.Errorf("warmed cache code = %q, want %q", cache.setCalls[0].code, "abc1234")
	}
}

func TestRedirectHandler_NotFound(t *testing.T) {
	repo := &stubRepository{url: nil, err: ports.ErrNotFound}
	cache := &stubCache{getHit: false}

	svc := service.NewURLService(repo, nil, cache)
	handler := redirectHandler(svc, newTestStatsBuffer())

	req := newTestRequest("doesnotexist")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRedirectHandler_RepoErrorMapsTo500(t *testing.T) {
	repo := &stubRepository{url: nil, err: errors.New("boom")}
	cache := &stubCache{getHit: false}

	svc := service.NewURLService(repo, nil, cache)
	handler := redirectHandler(svc, newTestStatsBuffer())

	req := newTestRequest("abc1234")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRedirectHandler_RecordsHitOnSuccess(t *testing.T) {
	repoURL := &domain.URL{Code: "abc1234", LongURL: "https://example.com/tracked"}
	repo := &stubRepository{url: repoURL, err: nil}
	cache := &stubCache{getHit: false}
	svc := service.NewURLService(repo, nil, cache)

	var captured map[string]int64
	sb := stats.NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error {
			captured = deltas
			return nil
		},
		time.Minute,
	)

	handler := redirectHandler(svc, sb)
	req := newTestRequest("abc1234")
	rec := httptest.NewRecorder()
	handler(rec, req)

	// force a flush to inspect what was buffered — Start() was never called,
	// so this is the only way to observe RecordHit's effect deterministically
	sb.Stop(context.Background())

	if captured["abc1234"] != 1 {
		t.Errorf("captured hits for abc1234 = %d, want 1", captured["abc1234"])
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
