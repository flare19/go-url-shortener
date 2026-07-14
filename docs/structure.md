# Project Structure

```
go-url-shortener/
├── cmd/
│   ├── writer/          # entrypoint for the Writer service (POST /shorten)
│   └── redirector/      # entrypoint for the Redirector service (GET /{code})
├── internal/
│   ├── domain/          # URL entity, validation rules — no external deps
│   ├── ports/           # interfaces: URLRepository, Encoder, (Cache)
│   ├── service/         # orchestrates domain + ports; composition point.
│   └── adapters/        # concrete implementations of the ports
│       ├── mongo/       # MongoURLRepository
│       └── encoding/    # Base62Encoder (or whichever strategy is chosen)
├── docs/
│   └── adr/             # architecture decision records
├── go.mod
└── README.md
```

## Layering rule

- `domain` depends on nothing else in this repo.
- `ports` defines interfaces `domain`/services need; depends only on `domain`.
- `adapters` implements `ports` interfaces; depends on `domain` + `ports` +
  external libs (mongo driver, etc).
- `cmd/*` wires concrete adapters into services and starts the HTTP server.
  This is the only place concrete adapter types should be referenced —
  everything else should talk to the `ports` interfaces.

_(update this file if the actual package layout diverges once code exists)_