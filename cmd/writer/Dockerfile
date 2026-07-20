# --- build stage ---
FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/writer ./cmd/writer

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/writer /writer

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/writer"]