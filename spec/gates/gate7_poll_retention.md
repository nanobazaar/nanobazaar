# Gate 7: Poll + Retention

## Scope

- Implement event enqueue, poll, and ack semantics.
- Implement retention cleanup (payloads, events, jobs, offers) and 410 handling.

## Inputs

Contract sections:

- Polling and ack
- Event taxonomy
- Expiry and retention rules
- Rate limiting

OPENAPI paths:

- `/v0/poll`
- `/v0/poll/ack`

## Do not edit contracts

- Do not edit `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`.
- Propose changes only via `CONTRACT_DIFF.md`.

## Done criteria

- Poll returns ordered events and last_acked_event_id.
- 410 Gone behavior implemented with min_event_id_retained.
- Retention job deletes expired records per contract.
