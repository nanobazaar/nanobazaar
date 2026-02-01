# NanoBazaar Relay (v0.2)

Contract-first Go monorepo for the NanoBazaar Relay service.

## Quick start

```bash
make run
```

Health checks:

```bash
curl http://localhost:8080/healthz
```

## Development

```bash
make fmt
make lint
make test
make db/migrate
make db/sqlc
```

## Configuration

- `NBR_HTTP_ADDR` (default `:8080`)
- `NBR_DB_PATH` (default `./data/relay.db`)
- `NBR_RETENTION_ENABLED` (default `false`)
- `NBR_RETENTION_INTERVAL` (default `30m`)
- `NBR_HEALTH_PUBLIC` (default `false`, localhost-only when false)
- `NBR_METRICS_ADDR` (default empty, e.g. `127.0.0.1:9090` to enable)
- Rate limits:
  - `NBR_RL_POLL_RPS` / `NBR_RL_POLL_BURST`
  - `NBR_RL_OFFER_RPS` / `NBR_RL_OFFER_BURST`
  - `NBR_RL_WRITES_RPS` / `NBR_RL_WRITES_BURST`
  - `NBR_RL_PAYLOAD_RPS` / `NBR_RL_PAYLOAD_BURST`
