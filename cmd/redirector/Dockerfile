# --- build stage ---
FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/redirector ./cmd/redirector

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/redirector /redirector

USER nonroot:nonroot
EXPOSE 8081

ENTRYPOINT ["/redirector"]