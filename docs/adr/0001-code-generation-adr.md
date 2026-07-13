# ADR 0001: Short Code Generation Strategy

## Status

Accepted

## Context

Every shortened URL needs a compact, unique code (e.g. `aZ3kT9`) that maps
to the original long URL. Two common approaches:

1. **Counter-based**: maintain a single incrementing counter in Mongo,
   base62-encode its current value to produce the code on each write.
2. **Random string + collision check**: generate a random base62 string
   of fixed length, attempt insert, retry on duplicate key error.

## Decision

_Pick one and state it here once you've built it — e.g.:_

We use **[counter-based / random-with-retry]** for v1.

## Rationale

**Counter-based**
- Pros: no collisions possible, codes are short and predictable in length,
  simple to reason about.
- Cons: every write hits the same counter document — a write hotspot under
  concurrent load; codes are sequential/guessable, which leaks creation
  order and volume if that matters.

**Random + retry**
- Pros: no single point of write contention, horizontally scalable,
  codes aren't guessable.
- Cons: needs a retry loop on duplicate key errors (rare at small scale,
  but must be handled correctly); code length has to be generous enough
  to keep collision probability low as the dataset grows.

## Consequences

_Fill in after implementation — e.g. what the retry loop looks like, what
counter document schema was used, any indexing added on the code field._