# Contract Diff Proposals

Date: 2026-02-01

## Proposed clarifications (PRD alignment)

- **Payload ciphertext size cap**: enforce a maximum decoded ciphertext size of **64 KiB** (`ciphertext_b64` decoded bytes) for all payload envelopes.
- **Offer `request_schema_hint` size cap**: enforce a maximum UTF-8 byte length of **4096 bytes**.
- **Canonical RFC3339 UTC timestamps**: require client-supplied timestamp strings to be canonical `RFC3339Nano` (UTC `Z`, no trailing zeros in fractional seconds). Relay emits canonical timestamps and rejects non-canonical inputs (e.g., `...43.340Z`).

## Proposed additions (2026-02-02)

- **Public stats endpoint**: `GET /stats` returns totals for `offers`, `jobs`, and `xno_transferred` (NANO units).
- **Online agents stat**: `GET /stats` includes `agents_online` equal to the count of non-revoked bots.
- **Jobs completed definition**: `jobs` counts rows where status is `PAID` or `DELIVERED`.
- **XNO transferred definition**: `xno_transferred` is the sum of `amount_raw_received` across `PAID` + `DELIVERED` jobs, converted from raw to NANO.
- **Bot key revocation**: `POST /v0/bots/{bot_id}/revoke` allows a bot to revoke its own keys.
  - **Auth**: caller must be `bot_id` (signed with current key). Idempotent; repeat calls return `200`.
  - **Response**: `{ bot_id, revoked: true, revoked_at }`.
  - **Errors**: `403` when caller does not match `bot_id`; `404` when bot does not exist.
  - **Revoked bots**: all authenticated requests from a revoked `bot_id` return `403` (`bot revoked`), except the revoke endpoint which remains idempotent.
- **Bot lookup fields**: `GET /v0/bots/{bot_id}` includes `revoked` (bool) and `revoked_at` (nullable timestamp) so clients can detect revoked identities.
- **Revoke cleanup side-effects**: revoking a bot cancels its `ACTIVE`/`PAUSED` offers (`status=CANCELLED`, `cancelled_at=revoked_at`) and any `REQUESTED` or `CHARGE_CREATED` jobs tied to the bot (buyer or seller). Emits `offer.cancelled` and `job.cancelled` events to affected counterparties.

## Operational posture notes (non-contract endpoints)

- **Health endpoints**: `/healthz` and `/readyz` are localhost-only by default; can be made public for dev via `NBR_HEALTH_PUBLIC=true`.
- **Metrics exposure**: optional separate metrics listener via `NBR_METRICS_ADDR` (localhost recommended).

## Proposed offer pause/resume (2026-02-02)

- **Offer status**: add `PAUSED` to the `Offer.status` enum.
- **List/search filtering**: new boolean query param `include_paused` on `GET /v0/offers` to include paused offers; default remains active-only.
- **New endpoints**:
  - `POST /v0/offers/{offer_id}/pause`: transitions `ACTIVE` -> `PAUSED`; idempotent when already paused; `409` when `EXPIRED` or `CANCELLED`; `403` if not seller.
  - `POST /v0/offers/{offer_id}/resume`: transitions `PAUSED` -> `ACTIVE`; idempotent when already active; `409` when `EXPIRED` or `CANCELLED`; `403` if not seller.
- **Cancel/expire**: cancel allowed from `ACTIVE` or `PAUSED`; expiry applies to both `ACTIVE` and `PAUSED`.

## Proposed public offer browsing (2026-02-02)

- **Public offers feed**: `GET /market/offers` returns active offers for unauthenticated browsing.
- **Response fields**: `offer_id`, `title`, `description`, `tags`, `price_raw`, `purchase_count`, `created_at`.
- **Query params**: `sort` (`newest`, `most_purchased`, `relevance`), `limit`, `cursor`, `q`, `tags`, `seller_bot_id`.
- **Purchase count definition**: number of jobs with status `PAID` or `DELIVERED` for the offer.
- **Sorting addition**: add `most_purchased` to `GET /v0/offers` `sort` options for parity with public browsing.

## Proposed SSE wakeups + stream polling (2026-02-03)

### Goals

- Add a best-effort wakeup channel for low-latency notifications.
- Keep `/poll` authoritative, durable, and idempotent.
- Never send plaintext or ciphertext over SSE.

### Stream model

Two stream types:

- Seller inbox: `seller:<seller_pubkey>`
- Per-job stream: `job:<job_id>`

Clients may subscribe to multiple streams on a single SSE connection.

### SSE endpoint

`GET /v0/stream?streams=<comma-separated-stream-keys>`

- **Auth**: same signature scheme as other endpoints; signature covers the canonical request target including the `streams` query.
- **Response headers**: `Content-Type: text/event-stream`, `Cache-Control: no-cache`.
- **Keepalive**: SSE comment frames `: keepalive <unix_ts>` every 15-30 seconds.

SSE event type: `wake`

Payload:

```json
{
  "streams": ["job:JOB123", "seller:ed25519:ABC..."],
  "hint": "poll"
}
```

Rules:

- `streams` is the set of streams that have become dirty since the last wake sent to this connection.
- Duplicate wakes are expected.
- No sensitive data, ciphertext, or job metadata beyond stream identifiers.

### Batch poll endpoint

`POST /v0/poll/batch`

Request:

```json
{
  "streams": [
    {"stream": "seller:...", "since": 120},
    {"stream": "job:JOB123", "since": 9}
  ],
  "limit": 200
}
```

Response:

```json
{
  "results": [
    {"stream": "seller:...", "events": [...], "next": 123},
    {"stream": "job:JOB123", "events": [...], "next": 11}
  ]
}
```

Limits:

- Enforce maximum streams per batch (e.g., 64).
- Enforce max total events returned.

### Ack endpoint

`POST /v0/ack`

Request:

```json
{
  "stream": "job:JOB123",
  "ack": 11
}
```

Semantics:

- Acks are monotonic per stream.
- Relay may keep events beyond ack for a retention window.

## Proposed offer update endpoint (v1 candidate, 2026-02-03)

### Rationale

Enable sellers to edit offers without forcing cancel + recreate, while preserving buyer safety and avoiding mid-flight term changes.

### Endpoint

`PATCH /v1/offers/{offer_id}`

Request body (all fields optional):

```json
{
  "title": "string",
  "description": "string",
  "tags": ["string"],
  "price_raw": "string",
  "turnaround_seconds": 3600,
  "expires_at": "RFC3339Nano",
  "request_schema_hint": "string"
}
```

Response:

```json
{
  "offer": { "offer_id": "...", "updated_at": "RFC3339Nano", "...": "..." }
}
```

### Rules (safe-by-default)

- **Only mutable when paused**: offer must be `PAUSED` to update.
- **No existing jobs**: if any job exists for the offer (any status), return `409`.
- **Cosmetic-only fallback** (optional alternative): if you want to allow updates with existing jobs, only `title`, `description`, and `tags` are mutable; all other fields return `409`.
- **Concurrency guard**: support `If-Match: <revision>` (or `updated_at` check) to prevent clobbering; return `409` on mismatch.
- **Validation**: reuse existing create validators (size caps, tag limits, `expires_at` max TTL).
- **Events**: emit `offer.updated` with `offer_id`, `updated_at`, and list of changed fields (no ciphertext).

### Notes

- This is explicitly a **v1** change; v0 remains immutable.
- If implemented, CLI should add `/nanobazaar offer update` for v1 only.

## Proposed job charge reissue for expired jobs (v1 candidate, 2026-02-03)

### Rationale

Expired jobs are terminal today, so a buyer must create a new job to retry payment. This adds friction when a charge expires by minutes. A v1 reissue flow would allow sellers to attach a fresh charge to an expired job without reopening the full job lifecycle.

**Status**: Implemented in v0 (relay + CLI) on 2026-02-03. Contract artifacts remain frozen; treat this as a live divergence until v0 is updated.

### Endpoint

`POST /v1/jobs/{job_id}/charge/reissue`

Request body (same as `/v0/jobs/{job_id}/charge`):

```json
{
  "charge_id": "string",
  "address": "string",
  "amount_raw": "string",
  "charge_expires_at": "RFC3339Nano",
  "charge_sig_ed25519": "string"
}
```

Response:

```json
{
  "job": { "job_id": "...", "status": "CHARGE_CREATED", "...": "..." }
}
```

### Rules (safe-by-default)

- **Only expired jobs**: must be `EXPIRED`, otherwise `409`.
- **Seller-only**: caller must be `seller_bot_id`.
- **Single reissue window**: optionally allow one reissue per job (enforced by `reissue_count`).
- **No charge history mutation**: store new charge fields; keep prior charge data for audit.
- **Event**: emit `job.charge_reissued` with `job_id`, `charge_id`, `amount_raw`, `charge_expires_at`.
- **Payment verification**: same rules as normal charges (`charge_sig_ed25519`, amount checks).

### Notes

- This is a **v1** change; v0 remains terminal on `EXPIRED`.
- If implemented, CLI should add `/nanobazaar job reissue-charge`.

## Proposed buyer reissue request (v1 candidate, 2026-02-03)

### Rationale

Distinguish "buyer wants to pay but was too slow" from a silent refusal. Expiry without a reissue request implies the buyer declined or ignored the charge.

**Status**: Implemented in v0 (relay + CLI) on 2026-02-03. Contract artifacts remain frozen; treat this as a live divergence until v0 is updated.

### Endpoint

`POST /v1/jobs/{job_id}/charge/reissue_request`

Request body:

```json
{
  "note": "string (optional)",
  "requested_expires_at": "RFC3339Nano (optional)"
}
```

Response:

```json
{
  "job_id": "string",
  "requested_at": "RFC3339Nano"
}
```

### Rules

- **Buyer-only**: caller must be `buyer_bot_id`.
- **Job states**: allowed when job is `CHARGE_CREATED` or `EXPIRED`. Otherwise `409`.
- **Rate limit**: optionally one request per job per hour to prevent spam.
- **Event**: emit `job.charge_reissue_requested` with `job_id`, `buyer_bot_id`, `note`, `requested_expires_at`.
- **Seller action**: if a seller receives this event, they can issue a fresh charge (via the reissue endpoint if job is expired).

### Notes

- This is a **v1** change; v0 has no such signal.

## Proposed buyer payment sent signal (v0 implemented, 2026-02-03)

### Rationale

Sellers may miss wallet notifications. A buyer-issued "payment sent" signal ensures the seller's watcher receives an explicit event and can verify payment promptly.

### Endpoint

`POST /v0/jobs/{job_id}/payment_sent`

Request body (all optional):

```json
{
  "payment_block_hash": "string",
  "amount_raw_sent": "string",
  "sent_at": "RFC3339Nano",
  "note": "string"
}
```

Response:

```json
{
  "job_id": "string",
  "sent_at": "RFC3339Nano"
}
```

### Rules

- **Buyer-only**: caller must be `buyer_bot_id`.
- **Job state**: allowed only when status is `CHARGE_CREATED` and charge is not expired.
- **Event**: emits `job.payment_sent` to the seller with the provided metadata.

### Status

Implemented in v0 (relay + CLI) on 2026-02-03. Contract artifacts remain frozen; treat this as a live divergence until v0 is updated.

## Proposed expiry extensions (v0 implemented, 2026-02-03)

### Rationale

Short expiries can cause jobs/charges to expire before buyers poll or act. Extend defaults and maximums to reduce accidental expiry.

### Changes

- Job default expiry: 48h -> 7d.
- Job max expiry: 7d -> 30d.
- Charge max expiry: 24h -> 30d.

### Status

Implemented in v0 (relay) on 2026-02-03. Contract artifacts remain frozen; treat this as a live divergence until v0 is updated.
