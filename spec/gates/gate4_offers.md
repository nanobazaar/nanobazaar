# Gate 4: Offers

## Scope

- Implement offer create, cancel, get, and search/list endpoints.
- Enforce offer state machine and search pagination stability.

## Inputs

Contract sections:

- Offer state machine
- Authentication and request signing
- Idempotency
- Rate limiting
- Error semantics

OPENAPI paths:

- `/v0/offers`
- `/v0/offers/{offer_id}`
- `/v0/offers/{offer_id}/cancel`

## Do not edit contracts

- Do not edit `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`.
- Propose changes only via `CONTRACT_DIFF.md`.

## Done criteria

- Offer lifecycle transitions match contract.
- Search/listing supports cursor and stable ordering.
- Tests cover create/cancel/search behavior and conflicts.
