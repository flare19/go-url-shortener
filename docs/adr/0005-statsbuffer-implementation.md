# ADR 0005: StatsBuffer Implementation — Swap-on-Flush, FlushFunc Injection, Configurable Interval

## Status

Accepted

## Context

ADR 0002 established the decision to buffer hit counts in memory and flush
them asynchronously on a ticker, guarded by a mutex, rather than writing to
Mongo synchronously on every redirect. This ADR documents the concrete
implementation (`internal/adapters/stats.StatsBuffer`) and records three
places where the implementation makes a specific choice ADR 0002 left open,
or diverges from what ADR 0002 sketched.

## Decision

`StatsBuffer` is implemented as a concrete struct in
`internal/adapters/stats`, not as an interface under `internal/ports` as
ADR 0002's phrasing (`internal/ports.StatsBuffer`) implied. It is
constructed with a `FlushFunc` — a plain function value
(`func(ctx context.Context, deltas map[string]int64) error`) — rather than
depending on a repository port or interface directly.

```go
type FlushFunc func(ctx context.Context, deltas map[string]int64) error

type StatsBuffer struct {
	mu       sync.Mutex
	hits     map[string]int64
	flush    FlushFunc
	interval time.Duration
	done     chan struct{}
	wg       sync.WaitGroup
}
```

At construction (in `cmd/redirector`'s composition root), `repo.IncrementHitsBatch`
— a method already defined on `ports.URLRepository` — is passed directly as
the `FlushFunc`:

```go
statsBuffer := stats.NewStatsBuffer(repo.IncrementHitsBatch, config.StatsFlushInterval())
```

Flushing uses a double-buffer swap rather than draining the live map in
place:

```go
func (s *StatsBuffer) swap() map[string]int64 {
	s.mu.Lock()
	old := s.hits
	s.hits = make(map[string]int64)
	s.mu.Unlock()
	return old
}
```

The flush interval is read from `STATS_FLUSH_INTERVAL` (default `5s`) via
`internal/config`, not hardcoded as a constant.

## Rationale

- **Concrete struct + `FlushFunc`, not a `ports.StatsBuffer` interface:**
  `StatsBuffer` has exactly one real implementation and no need for test
  doubles beyond substituting the `FlushFunc` itself — the function value
  already gives tests full control over flush behavior (see
  `buffer_test.go`) without the ceremony of defining and satisfying an
  interface. `ports.URLRepository.IncrementHitsBatch` already exists and
  already matches `FlushFunc`'s signature exactly, so `StatsBuffer` takes a
  function value instead of a second, redundant interface type. This is a
  narrower dependency than accepting the whole `ports.URLRepository` — a
  compile-time signal that `StatsBuffer` cannot call `Save` or `FindByCode`,
  only the one method it actually needs.

- **Swap over in-place drain:** an in-place drain (iterate the map, call
  `IncrementHitsBatch`, then clear it) would need to hold the mutex for the
  duration of the Mongo write, or accept a race between clearing and new
  writes landing mid-clear. The swap instead replaces the live map with a
  fresh one under the lock (a pointer reassignment, released in
  nanoseconds), then flushes the *old* map with no lock held at all. New
  `RecordHit` calls land in the fresh map uncontended while the Mongo write
  for the old map is in flight — the redirect path is never blocked by a
  flush, including a slow or failing one.

- **Env var over fixed constant:** ADR 0002 suggested starting with a fixed
  constant and only promoting to an env var "if actually needed." This was
  promoted immediately rather than deferred, because `internal/config`
  already existed with an established `EnvOrDefault` pattern (used for
  `MONGO_DB`, `MONGO_COLLECTION`, etc.) by the time StatsBuffer was built —
  following that existing convention cost one function and one line in
  `config.go`, cheaper than the deferred-promotion path ADR 0002 anticipated
  when no such pattern existed yet.

## Failure mode (extends ADR 0002)

On a `FlushFunc` error (e.g. a Mongo write failure), the batch is logged
and dropped — not retried, not requeued. Because the swap has already
happened by the time the write is attempted, the failed batch cannot
silently merge with the next interval's hits; it is cleanly lost, not
double-counted. This is consistent with ADR 0002's framing of hit counts as
informational, not correctness-critical, and keeps `StatsBuffer` itself
free of retry/backoff logic that would add complexity out of proportion to
what the data is worth.

## Consequences

- `StatsBuffer.Start(ctx)` / `Stop(ctx)` replace ADR 0002's proposed
  `StartFlusher` naming. `Start` launches the ticker goroutine; `Stop`
  closes the goroutine, waits for it to exit, then performs one final
  `flushNow` — satisfying ADR 0002's "final flush on graceful shutdown"
  consequence directly. `Stop` is wired into `cmd/redirector`'s shutdown
  sequence between `srv.Shutdown` (stops accepting new requests, so no
  further `RecordHit` calls can land) and `client.Disconnect` (Mongo must
  still be reachable for the final flush).
- `STATS_FLUSH_INTERVAL` is now a documented, operator-tunable env var
  (default `5s`) rather than a constant requiring a code change to adjust.
- No interface exists for `StatsBuffer` itself under `internal/ports` —
  if a second implementation or a test double swapping the whole buffer
  (not just its `FlushFunc`) is ever needed, that would be a new decision,
  not something this ADR anticipates.