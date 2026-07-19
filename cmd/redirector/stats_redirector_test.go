package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flare19/go-url-shortener/internal/domain"
	"github.com/flare19/go-url-shortener/internal/ports"
	"github.com/flare19/go-url-shortener/internal/service"
)

func TestStatsHandler_Success(t *testing.T) {
	repoURL := &domain.URL{
		Code:       "abc1234",
		LongURL:    "https://example.com/tracked",
		HitCounter: 42,
	}
	repo := &stubRepository{url: repoURL, err: nil}
	cache := &stubCache{getHit: false}

	svc := service.NewURLService(repo, nil, cache)
	handler := statsHandler(svc)

	req := newTestRequest("abc1234")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var resp statsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if resp.Code != "abc1234" {
		t.Errorf("Code = %q, want %q", resp.Code, "abc1234")
	}
	if resp.LongURL != repoURL.LongURL {
		t.Errorf("LongURL = %q, want %q", resp.LongURL, repoURL.LongURL)
	}
	if resp.HitCount != 42 {
		t.Errorf("HitCount = %d, want %d", resp.HitCount, 42)
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt is empty, want a formatted timestamp")
	}
}

func TestStatsHandler_NotFound(t *testing.T) {
	repo := &stubRepository{url: nil, err: ports.ErrNotFound}
	cache := &stubCache{getHit: false}

	svc := service.NewURLService(repo, nil, cache)
	handler := statsHandler(svc)

	req := newTestRequest("doesnotexist")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q — NotFound must stay JSON, not fall back to plain text like redirectHandler", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error body as JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error(`response body missing "error" key`)
	}
}

func TestStatsHandler_RepoErrorMapsTo500(t *testing.T) {
	repo := &stubRepository{url: nil, err: errors.New("boom")}
	cache := &stubCache{getHit: false}

	svc := service.NewURLService(repo, nil, cache)
	handler := statsHandler(svc)

	req := newTestRequest("abc1234")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestStatsHandler_DoesNotRecordHit(t *testing.T) {
	// Regression guard for the "should /stats count as a hit" decision
	// (ADR 0006): statsHandler must be a passive read. Since statsHandler
	// takes no *stats.StatsBuffer argument at all, this is really just
	// confirming the signature stays that way — if someone later adds a
	// StatsBuffer param "for consistency" with redirectHandler, this test
	// (and its signature check) should force that decision to be conscious.
	repoURL := &domain.URL{Code: "abc1234", LongURL: "https://example.com/tracked", HitCounter: 5}
	repo := &stubRepository{url: repoURL, err: nil}
	cache := &stubCache{getHit: false}

	svc := service.NewURLService(repo, nil, cache)
	handler := statsHandler(svc)

	req := newTestRequest("abc1234")
	rec := httptest.NewRecorder()
	handler(rec, req)

	var resp statsResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.HitCount != 5 {
		t.Errorf("HitCount = %d, want unchanged 5 (statsHandler must not increment)", resp.HitCount)
	}
	_ = context.Background() // keep context import used if you trim other tests later
}
