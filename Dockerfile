# Syntax-only Dockerfile: pure-Go SQLite (modernc.org/sqlite) means no CGO,
# so the runtime image is scratch-friendly; Debian slim keeps CA certs and a
# shell for debugging.

FROM golang:1.27 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /track ./cmd/track

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /track /track

ENV PORT=8080 \
    TRACK_DB_PATH=/data/track.db \
    TRACK_DATA_PATH=/data \
    TRACK_ENV=production \
    SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

EXPOSE 8080
VOLUME /data
ENTRYPOINT ["/track"]