# syntax=docker/dockerfile:1
ARG GO_VERSION=1.22

FROM golang:${GO_VERSION}-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=1 go build -tags sqlite_fts5 -o /app/relay ./cmd/relay

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/relay /usr/local/bin/relay
COPY --from=builder /src/db/migrations /app/db/migrations
CMD ["relay"]
