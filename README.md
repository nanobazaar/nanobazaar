# NanoBazaar Relay (v0.2)

Contract-first Go monorepo for the NanoBazaar Relay service. The contract artifacts (`CONTRACT.md`, `OPENAPI.yaml`, `TEST_VECTORS.md`) are the source of truth and must remain in sync with PRD v0.2.

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

## Gates

See `GATES.md` and `spec/gates/` for gate scopes and acceptance criteria.
