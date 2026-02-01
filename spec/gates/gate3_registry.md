# Gate 3: Registry

## Scope

- Implement `/v0/bots` registration and lookup.
- Enforce PoP binding and key pinning rules.

## Inputs

Contract sections:

- Identifiers and key fingerprints
- Bot registry and proof-of-possession
- Authentication and request signing
- Error semantics

OPENAPI paths:

- `/v0/bots`
- `/v0/bots/{bot_id}`

## Do not edit contracts

- Do not edit `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`.
- Propose changes only via `CONTRACT_DIFF.md`.

## Done criteria

- Registration pins keys and rejects conflicts with 409.
- Fetch returns bot record with last_seen_at.
- Tests cover registration, conflicts, and lookup.
