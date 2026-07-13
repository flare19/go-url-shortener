# ADR 0002: Asynchronous Hit-Count Recording

## Status

Accepted

## Context

Every redirect (`GET /{code}`) should increment a hit counter for that
code. The redirect path is the hottest, most latency-sensitive path in
the system — the whole rationale for splitting Writer/Redirector (see
README) is to keep this path fast. Writing the hit count to Mongo
synchronously on every redirect would add a network round-trip to every
single request on that path, undermining the reason the split exists.

## Decision

Hit counts are buffered in an in-memory map (`internal/ports.StatsBuffer`),
guarded by a `sync.Mutex`, and incremented synchronously in the redirect
handler (map write only, no I/O). A separate goroutine, started at service
boot, wakes on a `time.Ticker` interval, drains the buffer, and writes the
accumulated deltas to Mongo in a batch (one `$inc` per code instead of one
per request).

## Rationale

- **Why a goroutine, not just async-by-default Go behavior:** the
  goroutine decouples the DB write's timing from the request's timing.
  Without it, the write would either block the request (defeats the
  purpose) or not happen at all. This is the only place in v1 where a
  goroutine is doing real work — it isn't added elsewhere just to have
  concurrency in the codebase.
- **Why a mutex:** `net/http` runs each request on its own goroutine, so
  concurrent redirects for the same or different codes call `Increment`
  from multiple goroutines simultaneously. Go maps are not safe for
  concurrent read/write; the mutex protects the shared `counts` map from
  a data race. Verified with `go test -race` (fails without the lock,
  passes with it).
- **What this trades away:** hit counts can lag the true value by up to
  one flush interval (e.g. 5–10s) if the process crashes before a flush.
  Acceptable — hit counts are informational, not correctness-critical.

## Alternatives considered

- **Synchronous write on every redirect** — simplest, but reintroduces
  the exact latency the Writer/Redirector split was meant to avoid.
- **Fire-and-forget goroutine per request** (`go repo.IncrementHitCount(...)`
  inline in the handler) — removes blocking but does not batch writes;
  under load this is one Mongo write per redirect, just async, and adds
  goroutine overhead per request with no batching benefit.

## Consequences

- `StartFlusher` must be called once at service boot in `cmd/redirector`.
- On graceful shutdown, a final flush should run before exit to minimize
  lost hit counts (tie in with graceful shutdown handling).
- Flush interval is a tunable — start with a fixed constant, promote to
  an env var only if actually needed.