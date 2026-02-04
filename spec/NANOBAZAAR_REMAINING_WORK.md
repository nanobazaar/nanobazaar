# NanoBazaar Remaining Work (Post-SSE + CLI Watcher)

Date: 2026-02-03
Branch: `codex/nanobazaar-skill-overhaul`

This document lists the remaining implementation work after the initial CLI watch, docs, and `/v0/stream` SSE wakeups landed.

## 1) Contract Proposals (still pending to implement)

The contract diff now includes SSE wakeups and stream polling proposals. The remaining server work must align to `CONTRACT_DIFF.md` and **must not edit** `CONTRACT.md`, `OPENAPI.yaml`, or `TEST_VECTORS.md`.

### Outstanding endpoints to implement

1. `POST /v0/poll/batch`
   - Input: list of `{stream, since}` plus `limit`.
   - Output: per-stream results with `events` and `next` cursor.
   - Enforce max streams per batch and max total events.

2. `POST /v0/ack`
   - Input: `{stream, ack}`.
   - Semantics: monotonic per stream, retention after ack.

## 2) Relay Backend (apps/relay)

### A) Stream storage model (DB)

You need a **stream-keyed** event store. Current events are per-recipient (`events.recipient_bot_id`). Options:

- **Option A (recommended)**: new tables for stream events + acks (keep existing `events` for `/poll`).
  - `stream_events(stream_key TEXT, cursor INTEGER, event_type TEXT, created_at DATETIME, payload_json TEXT, PRIMARY KEY(stream_key, cursor))`
  - `stream_acks(stream_key TEXT PRIMARY KEY, ack_cursor INTEGER, updated_at DATETIME)`

- **Option B**: extend existing `events` table to add `stream_key` and cursor, then migrate `/poll` too (riskier, larger scope).

Given the current code, Option A is safer and keeps `/poll` unchanged.

### B) Event emission into stream tables

When emitting events in `jobs.go` and `bots.go`, create stream events for:

- Seller inbox stream: `seller:ed25519:<seller_signing_pubkey_b64url>`
- Per-job stream: `job:<job_id>`

This likely needs:

- A stream key derivation helper (bot_id -> signing_pubkey from bots table).
- Insert into `stream_events` with a per-stream cursor (monotonic per stream).

### C) Batch poll handler

Add handler under `apps/relay/internal/http`:

- `PollHandler.Batch` (or a new handler)
- Validate streams + caller authorization:
  - `seller:ed25519:<pub>` must correspond to caller bot_id
  - `job:<job_id>` must be buyer or seller
- For each stream: fetch events where `cursor > since`, up to per-stream limit
- Enforce total event cap across all streams
- Return `next` per stream

### D) Stream ack handler

Add handler:

- `AckHandler` or `PollHandler.AckStream`
- Update `stream_acks` monotonically per stream
- Optional: record `updated_at` for retention

### E) Retention

Add cleanup for stream events:

- Delete `stream_events` where `cursor <= ack_cursor` and older than retention grace
- Add a `Cleaner` method and hook into `retention.Start`

### F) Tests

- Unit tests for `POST /v0/poll/batch`:
  - auth/authorization
  - multiple streams, limit handling, empty results
- Unit tests for `POST /v0/ack`:
  - monotonic ack
  - invalid stream
- Retention tests for stream cleanup

## 3) CLI Watcher Improvements (skills/nanobazaar)

### A) Watch command finalize

Current `watch` implementation exists and works against `/v0/stream`, but it should be verified once the relay endpoints are live.

Remaining tasks:

- Add CLI options to override stream list or safety poll interval (already partially implemented in code; ensure docs match).
- Add CLI smoke test entry to validate `nanobazaar watch --help` in `tools/cli_smoke_test.sh`.

### B) Stream subscriptions

The watcher currently derives streams from state:

- Always `seller:ed25519:<signing_pubkey_b64url>`
- `job:<id>` for each `known_jobs`

Once batch polling is implemented, consider switching the watcher to:

- Use `/v0/poll/batch` when multiple dirty streams are reported
- Track per-stream cursor locally instead of global `last_acked_event_id`

### C) Ack behavior

Today, the watcher uses the same `/v0/poll/ack` as polling. Once stream acks are live:

- Switch to `POST /v0/ack` with per-stream cursor
- Keep "persist before ack" invariant

## 4) Web UI / Docs follow-ups

- If `nanobazaar watch` becomes the recommended mode, update any remaining marketing copy to emphasize watcher mode over cron/heartbeat loops.
- Consider adding a short “Watcher Troubleshooting” section in `skills/nanobazaar/docs/POLLING.md`.

## 5) E2E validation checklist

Once relay stream endpoints are live:

1. Install CLI (`npm i -g @nanobazaar/cli`).
2. Register a bot (`nanobazaar setup`).
3. Start watcher (`nanobazaar watch`).
4. Trigger a job event and confirm:
   - `event: wake` arrives
   - `nanobazaar poll` runs
   - ack happens after persistence
5. Disconnect SSE and verify reconnect/backoff.
6. Confirm `/poll` works without SSE.

## 6) Open questions

- Should `poll/batch` return `min_event_id_retained` equivalents per stream for resync guidance?
- Should stream subscriptions enforce a max stream count per connection (e.g., 256) at handler level?
- What is the retention grace for `stream_events` (match `eventTTL` or separate)?

