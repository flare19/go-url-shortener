# go-url-shortener

A minimal URL shortener written in Go, built to practice composition-based
design (interfaces over inheritance) and a CQRS-style service split.

## Problem

Given a long URL, generate a short, unique alias that redirects to it.
Reads (redirects) vastly outnumber writes (creations) and have a much
tighter latency budget, so the two paths are served by separate services
sharing one MongoDB instance.

## Architecture

```
                    ┌─────────────┐
   POST /shorten ─▶ │   Writer    │──┐
                    └─────────────┘  │
                                      ▼
                                 ┌─────────┐
                                 │ MongoDB │
                                 └─────────┘
                                      ▲
                    ┌─────────────┐  │
   GET  /{code}  ─▶ │ Redirector  │──┘
                    └─────────────┘
```

**Why split writer/redirector instead of one service:** redirect traffic
dominates request volume and needs to be fast; creation is comparatively
rare and can afford heavier validation. Splitting on that asymmetry keeps
each service simple and lets them scale independently later, without
introducing the complexity of a full microservices setup (separate DBs,
message queues, etc.) that this project doesn't need.

Each service follows a ports & adapters layout:

```
internal/
  domain/      # URL entity, validation rules
  ports/       # URLRepository, Encoder interfaces
  adapters/    # MongoURLRepository, Base62Encoder implementations
cmd/
  writer/      # POST /shorten
  redirector/  # GET /{code}, GET /{code}/stats
```

Services depend on interfaces (`ports`), not concrete implementations, so
storage or encoding can be swapped (e.g. for tests) without touching
business logic.

## API

| Method | Path             | Description                          |
|--------|------------------|---------------------------------------|
| POST   | `/shorten`       | Create a short code for a long URL   |
| GET    | `/{code}`        | Redirect to the original long URL    |
| GET    | `/{code}/stats`  | Return hit count for a short code    |

## Short code generation

See [docs/adr/0001-short-code-generation.md](docs/adr/0001-short-code-generation.md).

## Running locally

```bash
docker-compose up
```

_(fill in once compose file / env vars are settled)_

## Out of scope for v1

Cut deliberately to keep this a same-day build:

- Authentication / user accounts
- Rate limiting
- Analytics beyond a raw hit counter
- Redis caching layer on the redirect path (Mongo alone is fine at this scale)

## Stack

Go · MongoDB · (Redis, stretch goal only)