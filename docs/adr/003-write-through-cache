# ADR 0003: Write-Through Cache for Read-Your-Writes Consistency

## Status

Accepted

## Context

Both services currently share a single Mongo instance, so there is no
read/write consistency gap in v1 (see ADR discussion in README —
CQRS here means split by access pattern, not split by data store).

However, if the Redirector's reads are ever scaled horizontally against
Mongo read replicas (one primary, N secondaries), a real gap appears:
a client could create a URL via the Writer, then immediately hit the
Redirector, and land on a secondary that hasn't yet replicated the new
document — a "read-your-own-writes" consistency violation, surfacing as
a false 404 on a link created moments ago.

This is designed for now, ahead of actually needing it, so the interface
exists and is provably correct — without standing up a real replica set,
which is unnecessary infrastructure for this project's scale.

## Decision

Introduce a `ports.Cache` interface. On every successful write (`POST
/shorten`), the Writer populates the cache with the new `code -> long_url`
mapping (write-through, not write-behind — cache is populated
synchronously as part of the write, not lazily on first read). The
Redirector checks the cache before falling back to Mongo on every `GET
/{code}`.

Cache entries carry a short TTL (e.g. 60s) — long enough to absorb the
read-your-writes window immediately after creation, short enough that the
cache never becomes a second source of truth that can drift from Mongo.

v1 implementation of `ports.Cache` is a simple in-memory map with a
`sync.RWMutex` and lazy TTL expiry (check-on-read). The interface is
deliberately Redis-shaped (`Get`, `Set` with TTL, `Delete`) so a Redis
adapter can be substituted later without touching Writer or Redirector
service logic — this is the same ports/adapters composition pattern used
for `URLRepository` and `Encoder`.

## Rationale

- **Why solve this before it's a real problem:** it's cheaper to design
  the interface now, while the codebase is small, than to retrofit it
  after the Redirector is already coupled directly to Mongo reads.
- **Why in-memory and not Redis for v1:** no replicas exist yet, so
  there's nothing to actually protect against yet — Redis would be
  infrastructure with no current job. The `ports.Cache` interface is the
  part that matters; the backing implementation is free to be trivial
  until the day replicas exist.
- **Why TTL and not permanent cache:** a permanent cache risks silently
  serving stale data if a URL is ever updated or deleted (out of scope
  for v1, but the design shouldn't foreclose it). A short TTL bounds
  staleness without requiring active invalidation logic.
- **Why write-through and not read-through (populate on cache miss):**
  the gap this defends against exists in the seconds right after
  creation — the cache needs to already be warm at that moment, not
  populated on the first read that might itself be the one that misses.

## Alternatives considered

- **Always read from primary** — defeats the purpose of having replicas;
  rejected.
- **Mongo causal consistency sessions** (`afterClusterTime` / read
  concern majority) — the "correct" DB-level mechanism for this exact
  problem. Not implemented here because it requires a real replica set
  to be meaningful; noted as the production-grade alternative if this
  project ever runs against real replicas.
- **Accept the staleness** — legitimate for many products, but rejected
  here specifically because the cache is cheap to add and the freshly-
  created-link case is exactly the case a demo/interviewer is most likely
  to test by hand.

## Consequences

- `ports.Cache` must be defined alongside `URLRepository` and `Encoder`.
- Writer's create flow becomes: insert into Mongo → on success, write to
  cache. Cache write failure should not fail the request (cache is an
  optimization, not a correctness requirement) — log and continue.
- Redirector's read flow becomes: check cache → on miss, read Mongo →
  (optionally) repopulate cache on miss to help subsequent reads.
- This does not solve staleness for URL updates/deletes, since those are
  out of scope for v1. If added later, cache invalidation on write must
  be added at that point.