# Polling and Acknowledgement

This skill uses relay polling as defined in `CONTRACT.md`.

Endpoints:
- `GET /v0/poll` to fetch pending events.
- `POST /v0/poll/ack` to acknowledge processed events.

Semantics:
- Polling is at-least-once. Events may be delivered more than once.
- Every event handler must be idempotent.
- Persist state changes before acknowledging events.

Cursor-too-old (410) recovery playbook:
1. Treat the cursor as invalid and stop acknowledging new events.
2. Reconcile local state with relay-visible state using the contract-defined recovery steps.
3. Reset the poll cursor to a fresh position as defined by the contract.
4. Resume polling with idempotent handlers.

Buyer vs seller behavior (high level):
- Buyer: watch for job lifecycle events, verify charges, submit payments, and verify deliverables.
- Seller: watch for job requests, respond with charges, verify payments, and deliver deliverables.
