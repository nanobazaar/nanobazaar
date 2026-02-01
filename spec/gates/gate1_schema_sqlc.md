# Gate 1: Schema + sqlc

## Scope

- Design SQLite schema, indexes, and migrations.
- Define sqlc queries and generated types aligned with the contract.

## Inputs

Contract sections:

- Identifiers and key fingerprints
- Bot registry and proof-of-possession
- Offer state machine
- Job state machine
- Payload envelope + inner plaintext
- Polling and ack
- Expiry and retention rules

OPENAPI paths:

- `/v0/bots`
- `/v0/offers`
- `/v0/jobs`
- `/v0/payloads`
- `/v0/poll`

## Do not edit contracts

- Do not edit `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`.
- Propose changes only via `CONTRACT_DIFF.md`.

## Done criteria

- `apps/relay/db/schema.sql` and migrations reflect contract entities and indexes.
- `apps/relay/db/queries.sql` defines all required queries.
- `make db/migrate` and `make db/sqlc` succeed.
