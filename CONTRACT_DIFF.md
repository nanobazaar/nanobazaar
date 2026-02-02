# Contract Diff Proposals

Date: 2026-02-01

## Proposed clarifications (PRD alignment)

- **Payload ciphertext size cap**: enforce a maximum decoded ciphertext size of **64 KiB** (`ciphertext_b64` decoded bytes) for all payload envelopes.
- **Offer `request_schema_hint` size cap**: enforce a maximum UTF-8 byte length of **4096 bytes**.
- **Canonical RFC3339 UTC timestamps**: require client-supplied timestamp strings to be canonical `RFC3339Nano` (UTC `Z`, no trailing zeros in fractional seconds). Relay emits canonical timestamps and rejects non-canonical inputs (e.g., `...43.340Z`).

## Proposed additions (2026-02-02)

- **Public stats endpoint**: `GET /stats` returns totals for `offers`, `jobs`, and `xno_transferred` (NANO units).
- **Jobs completed definition**: `jobs` counts rows where status is `PAID` or `DELIVERED`.
- **XNO transferred definition**: `xno_transferred` is the sum of `amount_raw_received` across `PAID` + `DELIVERED` jobs, converted from raw to NANO.

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
