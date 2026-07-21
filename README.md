# go-url-shortener

A production-deployed URL shortener written in Go, built around a
CQRS-style service split and hexagonal architecture — and used as a
vehicle to practice composition-based design (interfaces over
inheritance), infrastructure-as-code, and OIDC-based CI/CD on Azure.

**Live:** https://urlshortfrontend.z29.web.core.windows.net/

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
                                 ┌──────────────┐
                                 │ MongoDB Atlas│
                                 └──────────────┘
                                      ▲
                    ┌─────────────┐  │
   GET  /{code}  ─▶ │ Redirector  │──┘
   GET  /{code}/stats
                    └─────────────┘
```

**Why split writer/redirector instead of one service:** redirect traffic
dominates request volume and needs to be fast; creation is comparatively
rare and can afford heavier validation. Splitting on that asymmetry keeps
each service simple and lets them scale independently, without the
complexity of a full microservices setup (separate DBs, message queues)
that this project doesn't need.

Each service follows a ports & adapters (hexagonal) layout:

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

| Method | Path             | Description                        |
|--------|------------------|-------------------------------------|
| POST   | `/shorten`       | Create a short code for a long URL |
| GET    | `/{code}`        | Redirect to the original long URL  |
| GET    | `/{code}/stats`  | Return hit count for a short code  |

## Short code generation

See [docs/adr/0001-short-code-generation.md](docs/adr/0001-short-code-generation.md).

## Deployment

Both services run as Docker containers on separate Azure VMs
(`Standard_B2ats_v2`), each fronted by its own Caddy instance handling
automatic TLS via Let's Encrypt. The frontend is a static React/Vite SPA
served from Azure Blob static website hosting. All infrastructure is
provisioned via Terraform.

```
                 ┌─────────────────────┐
  Browser ──────▶│ Blob Static Website │  (React/Vite SPA)
                 └──────────┬──────────┘
                             │
              ┌──────────────┴──────────────┐
              ▼                              ▼
   ┌───────────────────┐         ┌──────────────────────┐
   │ Caddy → Writer VM  │         │ Caddy → Redirector VM│
   │  (TLS, Let's       │         │  (TLS, Let's         │
   │   Encrypt)         │         │   Encrypt)            │
   └─────────┬──────────┘         └──────────┬────────────┘
             │                                │
             └───────────────┬────────────────┘
                              ▼
                     MongoDB Atlas (M0)
```

### CI/CD

- **Frontend** — on push to `main`, builds the Vite app and deploys to
  Azure Blob storage via OIDC (no stored cloud credentials).
- **Backend** — `go test`/`go vet` run on every push; on merge to `main`,
  both service images are built, pushed to GHCR, and rolled out to their
  respective VMs via SSH, followed by a health check. Authenticated to
  Azure via federated OIDC credentials scoped per-pipeline.

Infra cost is kept near-zero by deallocating both VMs between active
development sessions — static public IPs persist across deallocate/start
cycles, so hostnames never change.

## Verified in production

Load-tested with `hey` against the live deployment: the redirect path
sustained 1,100+ req/s with clean `302` responses under 25 concurrent
connections. Stress-testing the write path also surfaced a real,
documented latency characteristic under concurrent load — see open
issues rather than a claim of a flawless result.

## Running locally

```bash
docker-compose up
```

_(fill in once compose file / env vars are settled)_

## Out of scope for v1

Cut deliberately to keep scope tight:

- Authentication / user accounts
- Rate limiting
- Analytics beyond a raw hit counter
- Redis caching layer on the redirect path (Mongo alone is fine at this scale)
- Custom domain / CDN in front of the frontend (Blob's default HTTPS
  endpoint is sufficient for now)

## Stack

Go · MongoDB Atlas · Docker · Terraform · Azure (VMs, Blob Storage) ·
Caddy · React · TypeScript · Vite · GitHub Actions (OIDC)