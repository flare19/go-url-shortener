// cmd/writer/main_test.go
package main

import (
	"bytes"
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

// --- local stubs, scoped to this package's tests only ---

type stubRepository struct {
	saveErr error
}

func (s *stubRepository) Save(ctx context.Context, u *domain.URL) error {
	return s.saveErr
}

func (s *stubRepository) FindByCode(ctx context.Context, code string) (*domain.URL, error) {
	return nil, ports.ErrNotFound
}

func (s *stubRepository) IncrementHitsBatch(ctx context.Context, deltas map[string]int64) error {
	return nil
}

type stubEncoder struct {
	code string
	err  error
}

func (e *stubEncoder) Generate(ctx context.Context) (string, error) {
	return e.code, e.err
}

// --- tests ---

func TestCreateHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		repo       *stubRepository
		encoder    *stubEncoder
		wantStatus int
	}{
		{
			name:       "success",
			body:       `{"long_url":"https://example.com"}`,
			repo:       &stubRepository{},
			encoder:    &stubEncoder{code: "abc1234"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "malformed json",
			body:       `{not-json`,
			repo:       &stubRepository{},
			encoder:    &stubEncoder{code: "abc1234"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty long_url triggers domain validation error",
			body:       `{"long_url":""}`,
			repo:       &stubRepository{},
			encoder:    &stubEncoder{code: "abc1234"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "repo save failure maps to 500",
			body:       `{"long_url":"https://example.com"}`,
			repo:       &stubRepository{saveErr: errors.New("boom")},
			encoder:    &stubEncoder{code: "abc1234"},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewURLService(tt.repo, tt.encoder, nil) // cache nil — Create doesn't touch cache
			handler := createHandler(svc, "http://localhost:8081")

			req := httptest.NewRequest(http.MethodPost, "/shorten", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantStatus == http.StatusCreated {
				var resp createResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Code != "abc1234" {
					t.Errorf("code = %q, want %q", resp.Code, "abc1234")
				}
				if resp.ShortURL != "http://localhost:8081/abc1234" {
					t.Errorf("short_url = %q, want %q", resp.ShortURL, "http://localhost:8081/abc1234")
				}
				if resp.LongURL != "https://example.com" {
					t.Errorf("long_url = %q, want %q", resp.LongURL, "https://example.com")
				}
			}
		})
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
