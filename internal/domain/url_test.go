package domain

import (
	"errors"
	"testing"
)

func TestNewURL(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		longURL string
		wantErr bool
	}{
		{
			name:    "valid http url",
			code:    "aZ3kT9",
			longURL: "http://example.com/path",
			wantErr: false,
		},
		{
			name:    "valid https url",
			code:    "aZ3kT9",
			longURL: "https://example.com/path?query=1",
			wantErr: false,
		},
		{
			name:    "empty long url",
			code:    "aZ3kT9",
			longURL: "",
			wantErr: true,
		},
		{
			name:    "whitespace only long url",
			code:    "aZ3kT9",
			longURL: "   ",
			wantErr: true,
		},
		{
			name:    "missing scheme",
			code:    "aZ3kT9",
			longURL: "example.com/path",
			wantErr: true,
		},
		{
			name:    "unsupported scheme",
			code:    "aZ3kT9",
			longURL: "ftp://example.com/path",
			wantErr: true,
		},
		{
			name:    "no host",
			code:    "aZ3kT9",
			longURL: "http:///path",
			wantErr: true,
		},
		{
			name:    "malformed url",
			code:    "aZ3kT9",
			longURL: "http://%zz",
			wantErr: true,
		},
		{
			name:    "empty code",
			code:    "",
			longURL: "http://example.com",
			wantErr: true,
		},
		{
			name:    "whitespace only code",
			code:    "   ",
			longURL: "http://example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewURL(tt.code, tt.longURL)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewURL(%q, %q) expected error, got nil", tt.code, tt.longURL)
				}
				if !errors.Is(err, ErrInvalidURL) {
					t.Errorf("NewURL(%q, %q) error = %v, expected to wrap ErrInvalidURL", tt.code, tt.longURL, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewURL(%q, %q) unexpected error: %v", tt.code, tt.longURL, err)
			}
			if got == nil {
				t.Fatal("NewURL returned nil URL with nil error")
			}
			if got.Code != tt.code {
				t.Errorf("got.Code = %q, want %q", got.Code, tt.code)
			}
			if got.LongURL != tt.longURL {
				t.Errorf("got.LongURL = %q, want %q", got.LongURL, tt.longURL)
			}
			if got.HitCounter != 0 {
				t.Errorf("got.HitCount = %d, want 0", got.HitCounter)
			}
			if got.CreatedAt.IsZero() {
				t.Error("got.CreatedAt is zero value, expected it to be set")
			}
		})
	}
}
