# CLAUDE.md

Context for AI tooling (Claude Code, OpenCode, or any assistant working in
this repo). Read this before suggesting changes, especially structural
ones. This file states *active constraints*, not just history — treat
past decisions here as binding unless a newer ADR explicitly supersedes
them.

## What this project is

A URL shortener in Go, backed by MongoDB, built as a learning project with
two explicit goals: practice composition-based design (interfaces over
inheritance/embedding) after a prior tendency toward inheritance-style
thinking, and produce something with real architectural reasoning behind
it rather than a vibecoded CRUD app.

## Architecture — do not restructure without a new ADR

- **Two services, one shared MongoDB instance**: `Writer` (handles
  `POST /shorten`) and `Redirector` (handles `GET /{code}`,
  `GET /{code}/stats`). Split by access pattern (read-heavy redirect path
  vs. rare write path), not by business domain — there is only one
  domain (URL) in this project. See README and ADR discussion for the
  full rationale. **Do not merge these into one service** or split them
  further without a new ADR.
- **Layering, strict dependency direction**:
  `domain` → `ports` → `adapters` → `cmd` (wiring).
  - `domain`: pure logic, zero external dependencies, zero I/O.
  - `ports`: interfaces only (`URLRepository`, `Encoder`, `Cache`).
    Depends only on `domain`.
  - `adapters`: concrete implementations of `ports` interfaces
    (`MongoURLRepository`, `Base62Encoder`, in-memory `Cache`). Depends
    on `ports` + `domain` + external libs.
  - `cmd/writer`, `cmd/redirector`: wire concrete adapters into services,
    start HTTP servers. This is the **only** place concrete adapter
    types should be referenced directly — service logic elsewhere talks
    to `ports` interfaces, never concrete adapters.
  - **Never have `domain` or `ports` import `adapters`.** If you find
    yourself doing this, the layering is broken — stop and reconsider.

## Composition, not inheritance — this is deliberate, not incidental

- Go has no implementation inheritance; this project leans into that by
  composing services out of small interfaces (`URLService` holds
  `ports.URLRepository`, `ports.Encoder`, `ports.Cache` as fields,
  injected via constructor), not by building base-struct hierarchies.
- Do not introduce a "BaseService" or similar shared-base pattern to
  reduce duplication across services. If duplication appears, prefer
  extracting a shared helper function or a new small interface over
  introducing a shared base type.
- Struct embedding (Go's literal composition mechanism) should only be
  used where there's a genuine shared-field/shared-method relationship —
  don't manufacture embedding for its own sake.

## Router and stdlib choices (deliberate, not defaults to "fix")

- **Router**: `gorilla/mux`, chosen deliberately for consistency with
  prior learning (not because it was necessary at this endpoint count —
  `net/http`'s built-in routing would also suffice). Don't suggest
  swapping to `chi` or bare `net/http` routing.
- **Logging**: `log/slog` (structured), not `fmt.Println`/`log.Println`.
- **Config**: plain `os.Getenv` with defaults in a small `internal/config`
  package — no `viper` or similar.
- **Error responses**: consistent `{"error": "message"}` shape across
  both services on all 4xx/5xx.

## Explicitly deferred — do not add without a new/updated ADR

These were considered and deliberately excluded from current scope, not
overlooked:

- **Redis** — a `ports.Cache` interface exists (ADR 0003) specifically so
  Redis can be substituted later without touching service logic, but the
  current implementation is in-memory. Don't add a Redis client
  dependency unless ADR 0003 is superseded.
- **Multi-node Mongo / read replicas** — not running; ADR 0003 documents
  the design considered for handling replica staleness (write-through
  cache) if/when this changes.
- **Auth, rate limiting, analytics beyond a raw hit counter** — out of
  scope for v1, see README.
- **Message queues / event sourcing** — this project uses CQRS in the
  "split by access pattern, shared datastore" sense, *not* full CQRS +
  event sourcing. Don't introduce a queue between Writer and Redirector.

## Testing — see ADR 0004 for full rationale

- `domain`: plain table-driven unit tests, no mocks.
- Service logic (`URLService`): unit tests against hand-written fakes
  implementing `ports` interfaces — no real Mongo.
- `adapters/mongo`: integration tests via `testcontainers-go`, tagged
  `integration` build tag, run separately from the fast default suite.
- `StatsBuffer` (ADR 0002): must be run with `go test -race` at least
  once to verify mutex correctness — this isn't optional given the
  concurrency claims made in that ADR.

## Where to look before proposing a structural change

1. `docs/adr/` — check whether a decision already exists and why.
2. `docs/structure.md` — the intended package layout and layering rule.
3. `README.md` — problem statement, architecture diagram, out-of-scope
   list.

If a proposed change conflicts with an existing ADR, say so explicitly
and suggest writing a new ADR to supersede it — don't silently override
prior reasoning. 