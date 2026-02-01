# PRD v0.2 Alignment Plan (Remaining Gaps)

Date: 2026-02-01

This plan targets the remaining PRD gaps called out in the alignment check: operational hardening (rate limits, metrics/logging), validation caps (payload + request_schema_hint), poll last_acked_event_id semantics, search/indexing scale improvements, filter pushdown, and health endpoint auth posture. Contract artifacts remain frozen; any externally visible behavior not already captured should be proposed in `CONTRACT_DIFF.md`.

---

## 0) Decisions + confirmations (short, unblockers)

- Confirm numeric caps (PRD says size-limited but not explicit):
  - `request_schema_hint` max length (bytes vs chars).
  - `ciphertext_b64` max decoded bytes (and therefore max base64url length).
- Confirm rate-limit budgets per bucket and keying strategy:
  - Buckets required by PRD: poll/ack, offer search/list, writes, payload fetch.
  - Decide whether the limiter keys on `bot_id` only or (`bot_id`, IP) for abuse resistance.
- Decide metrics exposure model while honoring “no public unauth endpoints”:
  - Option A: authenticated `/v0/metrics`.
  - Option B: bind a separate admin listener to localhost via `NBR_METRICS_ADDR`.
- Confirm health endpoint posture:
  - If strict, move `/healthz` and `/readyz` under auth or restrict to localhost.

If any of the above changes the public surface (caps, headers, endpoints), add a short proposal to `CONTRACT_DIFF.md`.

---

## 1) Fix spec-conformance behavior (must-do)

### 1.1 Poll `last_acked_event_id` semantics

- Update `apps/relay/internal/http/poll.go` so `last_acked_event_id` always reflects stored ack state, even when `since_event_id` is supplied.
- Keep `since_event_id` as the query cursor only (exclusive semantics already implemented).
- Add tests in `apps/relay/internal/http/poll_test.go`:
  - With `since_event_id` provided and stored ack higher/lower.
  - With no stored ack (0 default).
  - 410 response should still include `min_event_id_retained`.

Definition of done:
- Poll response returns stored ack; query cursor uses `since_event_id` or stored ack if omitted.

### 1.2 Validation caps (payload + request_schema_hint)

- Add caps in `apps/relay/internal/http/jobs.go` (payloads) and `apps/relay/internal/http/offers.go` (request_schema_hint):
  - Extend `validatePayloadInput` to decode base64url and enforce max decoded bytes.
  - Extend `normalizeOfferCreate` to enforce max `request_schema_hint` length.
- Use constants (e.g., `maxPayloadBytes`, `maxRequestSchemaHintBytes`) defined near handlers.
- Add tests in `apps/relay/internal/http/jobs_test.go` and `apps/relay/internal/http/offers_test.go` for over-limit inputs.

Definition of done:
- Over-limit payloads and schema hints return 400 with clear errors.

---

## 2) Operational hardening (PRD “Operational concerns”)

### 2.1 Rate limiting with headers

- Add a limiter package (e.g., `apps/relay/internal/ratelimit`) using a token bucket (in-memory is OK for v0 single-instance).
- Configure four buckets with independent limits:
  - `poll_ack`: `GET /v0/poll`, `POST /v0/poll/ack`
  - `offer_search`: `GET /v0/offers`
  - `writes`: POST cancels/creates/updates for offers and jobs (and optionally `/v0/bots` registration)
  - `payload_fetch`: `GET /v0/payloads/{payload_id}` (consider `/v0/payloads` list here or in offer_search)
- Middleware integration in `apps/relay/internal/http/router.go`:
  - Route-scoped middleware for each bucket.
  - Ensure middleware runs after auth (so `bot_id` is known), unless we choose IP-based fallback.
- On 429, respond with:
  - `Retry-After: <seconds>`
  - `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` per PRD.
- Add env config and defaults in `apps/relay/cmd/relay/main.go` (e.g., `NBR_RL_POLL_RPS`, `NBR_RL_POLL_BURST`, etc.).
- Tests:
  - Unit tests for limiter math and header formatting.
  - HTTP handler tests for 429 + headers.

Definition of done:
- Each bucket enforces independent limits and emits required headers.

### 2.2 Metrics + logging

- Add a metrics layer (new `apps/relay/internal/metrics` package):
  - Counters for RPS/errors per endpoint, auth failures by reason, 410s, 409s, 429s.
  - Gauges for pending payloads and stored payload bytes.
  - Histograms or buckets for latency (per endpoint), poll lag, ack latency.
- Instrumentation points:
  - Auth middleware in `apps/relay/internal/auth/auth.go` for failure reasons.
  - Poll handler in `apps/relay/internal/http/poll.go` for lag/ack and 410 counts.
  - Payload handlers for stored bytes + pending counts.
  - Global HTTP middleware for request duration and status codes.
- Decide exposure (authenticated endpoint or localhost-only listener) and wire it in `apps/relay/internal/http/router.go` + `apps/relay/cmd/relay/main.go`.
- Logging updates:
  - Add structured log entries for auth failures, 410s, and rate-limit rejections.
  - Ensure logs never include ciphertext, plaintext, or raw signatures.

Definition of done:
- PRD minimum metrics are exposed and logging includes auth failure reasons and poll/ack insight.

---

## 3) Scale-oriented improvements (P1 hardening)

### 3.1 Filter pushdown

- Poll event type filtering:
  - Add SQLC query to filter by `event_type` in `apps/relay/db/queries.sql` (use `sqlc.slice` for IN).
  - Update `apps/relay/internal/http/poll.go` to call DB with types when provided.
- Job list status filtering:
  - Add SQLC queries for buyer/seller list paths with `status IN (...)` and with/without `created_since` + cursor.
  - Update `apps/relay/internal/http/jobs.go` to select the appropriate query based on filters, removing in-memory status filtering.
- Add test coverage for filtered queries and cursor behavior.

Definition of done:
- Poll + job list filtering is applied in SQL, not post-query, and existing semantics remain unchanged.

### 3.2 Offer search indexing

- Add SQLite FTS5 table (e.g., `offers_fts`) with triggers to keep `title`, `description`, and `tags` in sync.
- Update `apps/relay/db/schema.sql` + create a new migration in `apps/relay/db/migrations/`.
- Update `apps/relay/internal/http/offers.go` to use FTS for query scoring instead of in-memory scoring when `q` is supplied; keep exact behavior for non-query sorting.
- Ensure deterministic tie-breakers (existing cursor logic) remain stable.

Definition of done:
- Query searches are DB-driven and scale-friendly; sorting and cursor semantics remain stable.

---

## 4) Health endpoints posture

- If strict PRD stance applies, move `/healthz` and `/readyz` behind auth or make them localhost-only via middleware.
- Add config toggles for dev (e.g., `NBR_HEALTH_PUBLIC=true`).
- Update docs or `CONTRACT_DIFF.md` if public health endpoints remain.

Definition of done:
- Health endpoints comply with the “no public unauth endpoints” requirement (or have an approved exception).

---

## 5) Verification checklist

- Run `make fmt`, `make lint`, `make test`.
- If DB schema changes were made, run `make db/migrate` and `make db/sqlc`.
- Manual smoke checks:
  - Rate limit headers + Retry-After on 429.
  - Poll with/without since_event_id returns stored last_acked_event_id.
  - Over-limit payload + schema hint inputs reject with 400.
  - Metrics endpoint returns expected counters.

---

## 6) Suggested sequencing

1) Poll ack fix + validation caps (small, high impact, minimal deps)
2) Rate limiting middleware + config
3) Metrics/logging + exposure decision
4) Filter pushdown (poll + jobs)
5) Offer search indexing + migration
6) Health endpoint auth decision + doc updates

---

## 7) Files likely touched

- `apps/relay/internal/http/poll.go`
- `apps/relay/internal/http/jobs.go`
- `apps/relay/internal/http/offers.go`
- `apps/relay/internal/http/router.go`
- `apps/relay/internal/auth/auth.go`
- `apps/relay/internal/metrics/` (new)
- `apps/relay/internal/ratelimit/` (new)
- `apps/relay/db/schema.sql`
- `apps/relay/db/queries.sql`
- `apps/relay/db/migrations/*`
- `apps/relay/cmd/relay/main.go`
- `apps/relay/internal/http/*_test.go`
- `CONTRACT_DIFF.md` (if caps/headers/endpoints need formal proposal)
