# NanoBazaar Relay Gates (v0.2)

Each gate is independently implementable by a subagent. Contract files are frozen after Gate 0.

## Gate 0: Contract freeze

- Deliverables: CONTRACT.md, OPENAPI.yaml, TEST_VECTORS.md finalized; policy for CONTRACT_DIFF.md.
- Acceptance: contract artifacts are self-consistent and match PRD v0.2; no TODOs.

## Gate 1: Schema + sqlc

- Deliverables: SQLite schema, indexes, migrations, sqlc queries, generated types.
- Acceptance: `make db/migrate` and `make db/sqlc` succeed; schema supports contract entities.

## Gate 2: Auth middleware

- Deliverables: signature verification, replay protection, idempotency middleware.
- Acceptance: unit tests for signing/replay/idempotency; 401/409 behavior matches contract.

## Gate 3: Registry

- Deliverables: `/v0/bots` endpoints with PoP binding and key pinning.
- Acceptance: registration and fetch flows pass tests; 409 on conflicts.

## Gate 4: Offers

- Deliverables: offer create/cancel/search endpoints, search pagination rules.
- Acceptance: offer lifecycle tests; search sorting and cursor stability.

## Gate 5: Jobs

- Deliverables: job create/cancel/charge/mark_paid endpoints, state transitions, expiry checks.
- Acceptance: job state machine tests; conflict rules enforced.

## Gate 6: Payloads

- Deliverables: payload store/fetch/list endpoints and payload metadata.
- Acceptance: payload auth and idempotency tests; metadata listing works.

## Gate 7: Poll + retention

- Deliverables: event enqueue, poll/ack, retention cleanup job.
- Acceptance: poll/ack ordering; 410 behavior; retention deletion tests.

## Gate 8: Fly ops

- Deliverables: Fly.io deploy artifacts, volume mount, health checks.
- Acceptance: local run + health checks; fly config passes validation.
