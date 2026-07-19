# ADR 0006: /{code}/stats endpoint — JSON errors and eventual-consistency hit_count

## Status
Accepted

## Context
GET /{code}/stats was documented in README.md and docs/api-reference.md
but never implemented (see session handoff, pre-manual-test checklist).
Implementing it surfaced two design decisions.

## Decision 1: JSON error responses, diverging from redirectHandler
redirectHandler uses plain-text error responses (http.NotFound,
http.Error) since redirect clients don't parse response bodies.
statsHandler is a JSON API endpoint and returns JSON error bodies
({"error": "..."}) instead, matching writer's convention rather than
redirector's existing plain-text pattern. redirectHandler is left
unchanged — unifying redirector's error format entirely is tracked
separately (see GitHub issue: 404 responses aren't JSON).

## Decision 2: hit_count is eventually consistent, not real-time
statsHandler calls the existing URLService.Get, reused as-is rather
than adding a cache-bypassing read path. This means hit_count reflects:
  - Whatever was last flushed from StatsBuffer to Mongo
    (up to STATS_FLUSH_INTERVAL, default 5s, of buffered-but-unflushed
    hits not yet counted), plus
  - Whatever was cached at last cache miss (cache TTL, 60s, during
    which a Mongo-side update from IncrementHitsBatch is invisible to
    a warm cache entry, since Cache.Delete() is never called from any
    flush path).
  - Combined worst-case staleness: ~65s.

This is an accepted tradeoff, not a bug. hit_count is analytics data,
not a correctness-critical field — a short-lived undercount doesn't
affect redirect correctness, auth, or any other guarantee the system
makes. Bypassing the cache for /stats reads (e.g. always hitting
FindByCode directly) was considered and rejected: it would add
special-cased logic for one endpoint and undermine the cache's purpose
for what's likely to be a low-traffic, non-critical read path.

## Consequences
- /stats consumers should not assume hit_count is real-time.
- docs/api-reference.md's /stats section gets a one-line caveat noting
  possible lag, rather than promising real-time accuracy.