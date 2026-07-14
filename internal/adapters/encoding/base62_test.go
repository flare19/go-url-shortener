package encoding

import (
	"context"
	"strings"
	"testing"
)

func TestRandomEncoder_Generate_Length(t *testing.T) {
	enc := NewRandomEncoder()

	code, err := enc.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	if len(code) != codeLength {
		t.Errorf("len(code) = %d, want %d", len(code), codeLength)
	}
}

func TestRandomEncoder_Generate_AlphabetOnly(t *testing.T) {
	enc := NewRandomEncoder()

	code, err := enc.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	for _, ch := range code {
		if !strings.ContainsRune(alphabet, ch) {
			t.Errorf("code %q contains character %q not in alphabet", code, ch)
		}
	}
}

// TestRandomEncoder_Generate_Uniqueness is a statistical sanity check,
// not a proof of correctness — with a 62^7 keyspace, generating a
// modest number of codes should essentially never collide. If this
// test ever flakes, it's worth treating as a real signal, not a fluke
// to retry away, since it would suggest the randomness source or
// alphabet is smaller/weaker than assumed.
func TestRandomEncoder_Generate_Uniqueness(t *testing.T) {
	enc := NewRandomEncoder()
	seen := make(map[string]bool)

	const n = 10_000
	for i := 0; i < n; i++ {
		code, err := enc.Generate(context.Background())
		if err != nil {
			t.Fatalf("Generate returned unexpected error on iteration %d: %v", i, err)
		}
		if seen[code] {
			t.Fatalf("duplicate code generated: %q (after %d generations)", code, i)
		}
		seen[code] = true
	}
}
