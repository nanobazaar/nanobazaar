# Gate 6: Payloads

## Scope

- Implement payload store, fetch, and list endpoints.
- Enforce recipient authorization and metadata-only listing.

## Inputs

Contract sections:

- Payload envelope + inner plaintext
- Authorization rules
- Retention rules

OPENAPI paths:

- `/v0/payloads`
- `/v0/payloads/{payload_id}`

## Do not edit contracts

- Do not edit `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`.
- Propose changes only via `CONTRACT_DIFF.md`.

## Done criteria

- Ciphertext storage and fetch semantics implemented.
- Fetch marks fetched_at; listing returns metadata only.
- Tests cover auth and idempotency behavior.
