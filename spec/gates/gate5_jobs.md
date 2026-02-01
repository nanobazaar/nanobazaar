# Gate 5: Jobs

## Scope

- Implement job create/get/list/cancel/charge/mark_paid/deliver endpoints.
- Enforce job state machine, expiry triggers, and charge signature storage rules.

## Inputs

Contract sections:

- Job state machine
- Charge signature
- Expiry and retention rules
- Authorization rules
- Idempotency

OPENAPI paths:

- `/v0/jobs`
- `/v0/jobs/{job_id}`
- `/v0/jobs/{job_id}/cancel`
- `/v0/jobs/{job_id}/charge`
- `/v0/jobs/{job_id}/mark_paid`
- `/v0/jobs/{job_id}/deliver`

## Do not edit contracts

- Do not edit `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`.
- Propose changes only via `CONTRACT_DIFF.md`.

## Done criteria

- Job transitions and conflict rules match contract.
- Charge attach and mark_paid validations enforced.
- Tests cover major state transitions and 409 cases.
