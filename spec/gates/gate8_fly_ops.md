# Gate 8: Fly Ops

## Scope

- Validate Fly.io deployment configuration.
- Ensure health checks and DB volume configuration are correct.

## Inputs

- `apps/relay/deploy/fly.toml`
- `README.md` configuration section

## Do not edit contracts

- Do not edit `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`.
- Propose changes only via `CONTRACT_DIFF.md`.

## Done criteria

- Fly app starts with mounted volume at `/data`.
- Health checks hit `/healthz` on port 8080.
- Local run instructions remain valid.
