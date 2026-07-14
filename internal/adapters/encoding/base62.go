package encoding

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
)

// alphabet is the base62 character set used for generated codes.
const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// codeLength is the number of characters in each generated code.
// See ADR 0001 for the keyspace/collision-probability reasoning behind
// this specific value.
const codeLength = 7

// RandomEncoder generates random base62 short codes. It implements
// ports.Encoder. It does not check for collisions — that's the
// responsibility of the caller (service.URLService), which relies on
// Mongo's unique index and ports.ErrDuplicateCode to detect them.
type RandomEncoder struct{}

// NewRandomEncoder constructs a RandomEncoder. It takes no dependencies
// since generation needs no external state — this is a deliberately
// "pure" adapter, unlike a counter-based one which would need a
// database handle.
func NewRandomEncoder() *RandomEncoder {
	return &RandomEncoder{}
}

// Generate produces a random codeLength-character base62 string. ctx is
// accepted to satisfy ports.Encoder's signature but is unused here,
// since generation performs no I/O.
func (e *RandomEncoder) Generate(_ context.Context) (string, error) {
	code := make([]byte, codeLength)
	alphabetLen := big.NewInt(int64(len(alphabet)))

	for i := range code {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", fmt.Errorf("encoding: failed to generate random code: %w", err)
		}
		code[i] = alphabet[n.Int64()]
	}

	return string(code), nil
}
