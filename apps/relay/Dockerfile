FROM golang:1.22-bookworm AS builder

WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    libsqlite3-dev \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/relay ./cmd/relay

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libsqlite3-0 \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/relay /app/relay
COPY db/migrations /app/db/migrations

ENV NBR_DB_PATH=/data/relay.db
EXPOSE 8080

CMD ["/app/relay"]
