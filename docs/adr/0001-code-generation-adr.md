# ADR 0001: Short Code Generation Strategy

## Status

Accepted

## Context

Every shortened URL needs a compact, unique code (e.g. `aZ3kT9`) that maps
to the original long URL. Two approaches were considered:

1. **Counter-based**: maintain a single incrementing counter in Mongo,
   base62-encode its current value on each write.
2. **Random string + collision check**: generate a random base62 string
   of fixed length, attempt insert, retry on duplicate key error.

## Decision

We use **random generation with retry-on-collision**.

- Alphabet: base62 (`a-z`, `A-Z`, `0-9`)
- Code length: 7 characters → keyspace of 62⁷ ≈ 3.5 trillion codes
- Retry bound: 5 attempts (enforced in `service.URLService.Create`, not
  in the `Encoder` itself — see ADR discussion in `ports.go` comments:
  collision handling is the service's responsibility, not the encoder's)

## Rationale

**Why random over counter-based:**
- No write hotspot — random generation never contends on a single shared
  document the way an incrementing counter does under concurrent writes.
- Horizontally scalable by construction — multiple Writer instances can
  generate codes independently with no coordination needed.
- `URLService.Create` was already designed around a retry loop as a
  first-class path (not a bolted-on edge case), making random generation
  the more natural fit for the code as written.
- Codes aren't sequential/guessable, which avoids leaking creation order
  or volume — a minor but free benefit of this approach.

**Why the collision risk is acceptable:**
At 7 characters, keyspace is 62⁷ ≈ 3.52 × 10¹². Collision probability
for a single generation attempt ≈ (existing URL count) / (keyspace).
Even at 100 million stored URLs (far beyond this project's realistic
scale), that's under 3% per attempt — and the probability of colliding
on all 5 consecutive retry attempts is that figure raised to the 5th
power, effectively zero. 5 retries is a generous bound relative to the
actual risk, not a tight one.

**Why not counter-based:**
Simpler in one sense (collision-free by construction), but introduces a
write hotspot on a single counter document under concurrent writes, and
produces sequential, guessable codes. Rejected in favor of random given
the retry-loop design was already in place and the collision risk is
negligible at this keyspace size.

## Consequences

- `Encoder.Generate` (in `internal/adapters/encoding`) is a pure
  function — no database access, no counter document, no `ports`
  errors of its own. It generates a random string and returns it;
  it does not check uniqueness.
- Uniqueness is enforced entirely by Mongo's unique index on `code`,
  surfaced to `URLService` via `ports.ErrDuplicateCode`.
- If usage ever grows enough that collision rates become non-negligible
  at 7 characters, the fix is increasing code length, not switching
  strategy — worth revisiting this ADR's math if that ever becomes a
  real concern.