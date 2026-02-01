# Gate 2: Auth Middleware

## Scope

- Implement request signing verification.
- Enforce replay protection (timestamp window, nonce cache).
- Enforce idempotency guarantees and 409 conflict rules.

## Inputs

Contract sections:

- Authentication and request signing
- Replay protection
- Idempotency
- Error semantics

OPENAPI paths:

- All authenticated endpoints (`/v0/*`).

## Do not edit contracts

- Do not edit `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`.
- Propose changes only via `CONTRACT_DIFF.md`.

## Done criteria

- Middleware enforces signature, replay, and idempotency rules.
- Unit tests cover valid and invalid signature flows, replay, and idempotency collision.
