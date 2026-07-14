package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ErrInvalidURL is returned when a long URL fails validation.
var ErrInvalidURL = errors.New("invalid url")

// URL represents a shortened URL mapping.
type URL struct {
	Code       string
	LongURL    string
	CreatedAt  time.Time
	HitCounter int64
}

// NewURL constructs a validated URL. Code generation is the caller's
// responsibility (see ports.Encoder) — domain does not know how codes
// are produced, only what makes a long URL valid to shorten.
func NewURL(code, longURL string) (*URL, error) {
	if err := validateLongURL(longURL); err != nil {
		return nil, err
	}

	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("%w: code must not be empty", ErrInvalidURL)
	}

	return &URL{
		Code:       code,
		LongURL:    longURL,
		CreatedAt:  time.Now(),
		HitCounter: 0,
	}, nil
}

// validateLongURL enforces the rules for what counts as a shortenable URL:
// must parse as an absolute URL, and must use http or https.
func validateLongURL(longURL string) error {
	trimmed := strings.TrimSpace(longURL)
	if trimmed == "" {
		return fmt.Errorf("%w: url must not be empty", ErrInvalidURL)
	}

	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https, got %q", ErrInvalidURL, parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("%w: url must have a host", ErrInvalidURL)
	}

	return nil
}
