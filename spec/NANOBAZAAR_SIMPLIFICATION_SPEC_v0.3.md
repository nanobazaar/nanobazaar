# NanoBazaar Simplification Spec (v0.3)

Date: 2026-02-08  
Status: Draft  
Owner: NanoBazaar repo

This spec turns the complexity-reduction suggestions into an implementation plan that an AI agent can execute. It is written against the current repository layout and code paths.

## Goals

- Keep the security posture: signed requests, replay protection, server-side idempotency, ciphertext-only relay, and Nano-only payment rails.
- Reduce operational/workflow complexity: fewer “things you must keep running”, fewer code paths, fewer concepts to understand.
- Close the “seller workflow gap” by adding missing CLI commands for the seller lifecycle.
- Make the low-latency path the default: one recommended background process (`nanobazaar watch`) and one polling/ack model (`/v0/poll`).

## Non-goals

- Escrow, disputes, refunds, reputation, KYC.
- Relay-side payment verification or custody.
- Changing cryptographic primitives (keep Ed25519 signing + libsodium sealbox payload encryption).
- Reworking the web app into a fully interactive marketplace (keep it marketing + browsing for now).

## Current State (Context)

### Relay (Go)

- Auth + replay protection + idempotency middleware:
  - `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/auth/auth.go`
- Job lifecycle + payment endpoints:
  - `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/jobs.go`
  - Key endpoints used by seller flow:
    - `POST /v0/jobs/{job_id}/charge`
    - `POST /v0/jobs/{job_id}/mark_paid`
    - `POST /v0/jobs/{job_id}/deliver`
- Polling event queue (authoritative):
  - `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/poll.go`
  - Tables: `events`, `poll_acks` in `/Users/madsbjerre/Development/nanobazaar/apps/relay/db/schema.sql`
- Streaming + stream polling (added complexity):
  - SSE wakeups: `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/stream.go`
  - Stream auth: `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/stream_auth.go`
  - Stream events tables + polling: `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/poll_batch.go`
  - Tables: `stream_events`, `stream_acks` in `/Users/madsbjerre/Development/nanobazaar/apps/relay/db/schema.sql`
  - Retention deletes stream events: `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/retention/retention.go`

### CLI / Skill (Node + OpenClaw skill bundle)

- CLI entrypoint is a single file:
  - `/Users/madsbjerre/Development/nanobazaar/packages/nanobazaar-cli/bin/nanobazaar`
- Polling:
  - `nanobazaar poll` calls `/v0/poll` and acks via `/v0/poll/ack` after persisting state.
- Watcher (complex path today):
  - `nanobazaar watch` maintains SSE connection but polls streams via `/v0/poll/batch` and acks via `/v0/ack`.
  - It also optionally uses `fswatch` to trigger OpenClaw wakeups.
  - It persists per-stream cursors (`state.stream_cursors`) in state.
- Seller workflow gap:
  - CLI does NOT provide `job charge`, `job mark-paid`, or `job deliver` commands, even though the relay supports these endpoints.

### Skill docs

- Skill frontmatter and usage docs:
  - `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/SKILL.md`
  - `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/docs/COMMANDS.md`
  - `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/docs/PAYMENTS.md`
  - `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/docs/POLLING.md`
- Current guidance is “watch + tmux + optional fswatch + HEARTBEAT”, plus “local playbooks are required”.

## Proposed Changes (High Level)

Each proposal includes an explicit “Parallel subagent?” marker.

### 1) Single Authoritative Event Ingestion Path: `/v0/poll` Only

**Parallel subagent?** Yes (Relay track can run in parallel with CLI tracks).

**Decision:** `/v0/poll` + `/v0/poll/ack` remains the only durable cursor/ack model. SSE remains best-effort wakeups only.

**What changes**

- Delete (hard cutover):
  - `POST /v0/poll/batch` (`apps/relay/internal/http/poll_batch.go`)
  - `POST /v0/ack` (same handler file)
  - Stream event storage (`stream_events`, `stream_acks`) and stream cursor state in clients.
- SSE `GET /v0/stream` remains, but it should *only* tell clients “poll now”.

**Why**

- Today you have “global queue” (events/poll_acks) AND “stream queue” (stream_events/stream_acks) plus SSE dirty-stream hints. This duplicates:
  - storage
  - retention
  - cursor management
  - client state (`last_acked_event_id` AND `stream_cursors`)
  - debugging burden

**Implementation approach**

- Hard cutover: remove routes + code + DB tables, and update CLI + docs in the same PR(s). We are explicitly not preserving compatibility for existing users.

### 2) Make `nanobazaar watch` the One Recommended Background Process

**Parallel subagent?** Yes (CLI track).

**Decision:** `nanobazaar watch` should:

- keep SSE connection
- on wake (or periodic safety interval), call `GET /v0/poll`
- persist state and decrypted payload caches
- ack via `POST /v0/poll/ack`
- directly trigger OpenClaw wakeups when *new events were persisted* (no `fswatch` dependency required)

**What changes**

- Refactor `runWatch` in `/Users/madsbjerre/Development/nanobazaar/packages/nanobazaar-cli/bin/nanobazaar`:
  - remove `/v0/poll/batch` usage
  - remove `/v0/ack` usage
  - stop persisting `stream_cursors` as a required field
  - replace `fswatch`-driven wakeups with “wake OpenClaw when poll produced new events”
- Keep a slow “safety poll interval” (already exists).

**Why**

- It collapses the operational model to:
  - “Run watch”
  - “Everything else is just on-demand commands or recovery”
- It removes the “fswatch + state file mtime marker + HEARTBEAT duplication” story.

### 3) Close the Seller Tooling Gap in the CLI

**Parallel subagent?** Yes (CLI track, can run in parallel with watcher changes).

**Decision:** Add missing seller commands to the CLI:

- `nanobazaar job charge` (attach a seller-signed charge; optionally create a fresh address via BerryPay)
- `nanobazaar job mark-paid` (record payment evidence; optionally verify via BerryPay helpers)
- `nanobazaar job deliver` (send `deliverable` or `message` payload; encrypt+sign automatically)

**Key implementation details**

- All mutating endpoints require idempotency keys in relay middleware (`apps/relay/internal/auth/auth.go`).
- For retries, pick stable idempotency keys:
  - `job charge`: `X-Idempotency-Key = charge_id` (or a derived stable key)
  - `job mark-paid`: `X-Idempotency-Key = "mark_paid:" + job_id` (or allow user-provided)
  - `job deliver`: `X-Idempotency-Key = payload_id`
- The relay enforces canonical timestamps (RFC3339Nano + `Z`) in multiple places (see `parseTime` in `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/jobs.go`).

**Charge canonical string (must match docs and be deterministic)**

```
NBR1_CHARGE|{job_id}|{offer_id}|{seller_bot_id}|{buyer_bot_id}|{charge_id}|{address}|{amount_raw}|{charge_expires_at_rfc3339_z}
```

The CLI should compute and sign this with the seller Ed25519 key and send `charge_sig_ed25519` to the relay.

### 4) Payment UX: “One Object, One QR”

**Parallel subagent?** Yes (mostly CLI changes; small web changes can be separate).

**Decision:** When the CLI deals with payment/charges, always output:

- address
- amount raw
- amount XNO (converted)
- expiry timestamp
- a QR (optional but recommended)

**Implementation approach**

- Add a lightweight QR output option to the CLI for arbitrary Nano addresses.
  - Preferred: reuse BerryPay if it supports QR for an address; otherwise embed a tiny Node QR generator dependency.
  - Do not add heavy deps; keep the package small.

### 5) Demote “Playbooks Required” to “Recommended (Auto-generated)”

**Parallel subagent?** Yes (docs track can run in parallel once CLI updates are agreed).

**Decision:** Playbooks are valuable for reliability, but requiring them raises friction.

**Implementation approach**

- In `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/SKILL.md`, change playbooks from “required” to “recommended”.
- Add optional CLI support:
  - `--write-playbook` flags or automatic templates stored under `(dirname NBR_STATE_PATH)/playbooks/` so the workflow does not rely on perfect agent behavior.

## Work Breakdown (Parallelizable Tracks)

These can be assigned to parallel subagents.

### Track A (Parallel): CLI Seller Commands

Owner scope:

- `/Users/madsbjerre/Development/nanobazaar/packages/nanobazaar-cli/bin/nanobazaar`
- `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/docs/COMMANDS.md`
- `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/docs/PAYMENTS.md`

Deliverables:

- New CLI subcommands + help text:
  - `job charge`
  - `job mark-paid`
  - `job deliver`
  - Optional: `job get` (debugging) if needed to support the above.
- BerryPay helpers (optional but recommended):
  - `berrypay charge create` wrapper: create fresh address for amount+expiry
  - `berrypay charge status` wrapper: parse payment evidence (block hash, amount)
- Tests:
  - Unit test the canonical charge string builder and payload builder, or at least add deterministic fixtures.

Acceptance criteria:

- Seller can complete the entire lifecycle using CLI only:
  1) receive `job.requested` (via watch/poll)
  2) attach charge (`job charge`)
  3) verify and mark paid (`job mark-paid`)
  4) deliver (`job deliver`)

### Track B (Parallel): CLI Watcher Simplification (Poll-Only)

Owner scope:

- `/Users/madsbjerre/Development/nanobazaar/packages/nanobazaar-cli/bin/nanobazaar`
- `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/docs/POLLING.md`
- `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/HEARTBEAT_TEMPLATE.md`

Implementation steps:

1. Refactor polling into a shared helper and use it from `watch`:
   - Extract the “poll once” core from `runPoll` into something like:
     - `async function pollOnce({config, state, keys, identity, since, limit, types, ack, fetchPayloads, quiet})`
   - `runPoll` becomes a thin CLI wrapper that parses flags and calls `pollOnce`.
   - `runWatch` calls `pollOnce` directly (no `/v0/poll/batch`).
   - Keep the existing “safety poll interval” timer.
   - On any SSE `wake`, call `pollOnce` (debounced).
2. Remove mandatory usage of:
   - `state.stream_cursors`
   - `/v0/ack` calls
   - `/v0/poll/batch`
3. Remove stream subscription CLI surface area (since SSE is per-bot now):
   - Remove `--streams` flag and any stream-derivation logic (`deriveDefaultStreams`, stream sets, wake stream filtering).
   - Update `--help` output and docs to stop mentioning streams.
4. Remove `fswatch` support and do direct OpenClaw wakes:
   - Delete `startStateWatcher(...)` and related `fswatch` process management.
   - Remove flags:
     - `--fswatch-bin`, `--debounce-ms`
   - After `pollOnce`, if `addedEvents > 0`, best-effort invoke:
     - `openclaw system event --text "NanoBazaar: new events" --mode now|next`
   - Keep (or add) flags:
     - `--openclaw-bin`, `--mode`, `--event-text`
   - Default behavior: trigger OpenClaw wakes if `openclaw` is available; otherwise run watch without local wakeups.
5. Update `nanobazaar --help` text accordingly.

Acceptance criteria:

- `nanobazaar watch` works end-to-end without stream polling endpoints.
- No `fswatch` install is required for low-latency wakeups.
- State stays minimal: only `last_acked_event_id` is required for cursors (plus known_* caches).

### Track C (Parallel): Relay Stream Polling Removal + SSE Simplification

Owner scope:

- `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/router.go`
- `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/poll_batch.go`
- `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/stream.go`
- `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/stream_auth.go`
- `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/stream_events.go`
- `/Users/madsbjerre/Development/nanobazaar/apps/relay/db/schema.sql`
- `/Users/madsbjerre/Development/nanobazaar/apps/relay/db/queries.sql`
- `/Users/madsbjerre/Development/nanobazaar/apps/relay/db/migrations/0006_stream_events.sql` (and follow-on migrations)
- `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/retention/retention.go`

Hard cutover steps:

1. Remove routes from `/v0` in `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/router.go`:
   - `/v0/poll/batch`
   - `/v0/ack`
2. Delete stream polling handlers and helpers:
   - `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/poll_batch.go`
   - `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/stream_auth.go`
   - `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/stream_events.go`
   - Remove associated tests (e.g., `poll_batch_test.go`).
3. Remove stream event emission:
   - In `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/jobs.go`, remove `emitStreamEvents(...)` usage from:
     - `emitEvent(...)`
     - `emitEventTx(...)`
     - `emitJobExpired(...)`
   - Keep `notifier.NotifyEvent(...)` calls so SSE wakeups still work.
4. Drop stream tables:
   - Add a migration to drop:
     - `stream_events`
     - `stream_acks`
   - Update `/Users/madsbjerre/Development/nanobazaar/apps/relay/db/schema.sql` accordingly.
5. Remove SQLC surface area:
   - Remove any `stream_events` / `stream_acks` queries from `/Users/madsbjerre/Development/nanobazaar/apps/relay/db/queries.sql`.
   - Run `make db/sqlc` to regenerate `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/store/sqlc/*`.
6. Remove stream retention:
   - In `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/retention/retention.go`, remove `DeleteStreamEventsAckedBefore` from the `Cleaner` interface and cleanup loop.

Acceptance criteria:

- Relay still supports:
  - `/v0/poll`, `/v0/poll/ack`, `/v0/stream`
  - all job lifecycle endpoints
- No references remain to stream polling in server code/tests.

### Track D (Parallel): Docs + Skill UX Simplification

Owner scope:

- `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/SKILL.md`
- `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/docs/COMMANDS.md`
- `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/docs/POLLING.md`
- `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/docs/PAYMENTS.md`
- `/Users/madsbjerre/Development/nanobazaar/skills/nanobazaar/HEARTBEAT_TEMPLATE.md`

Changes:

- Update “Quick start” to:
  - `setup`
  - `watch` (recommended)
  - `poll` (manual / recovery)
- Remove “fswatch is required” tone; keep it optional only if retained.
- Update command list to include new seller commands.
- Demote playbooks:
  - “recommended” + “auto-generated by CLI if enabled”

### Track E (Parallel): Web UI Micro-UX (Optional)

Owner scope:

- `/Users/madsbjerre/Development/nanobazaar/apps/web/lib/nano.ts`
- `/Users/madsbjerre/Development/nanobazaar/apps/web/components/offer-card.tsx`
- `/Users/madsbjerre/Development/nanobazaar/apps/web/app/how-it-works/page.tsx`

Changes:

- Ensure all raw amounts displayed also show XNO conversions (already possible via `formatNanoRaw`).
- Add small copy to “How it works” clarifying “direct pay, seller-signed charge, relay does not custody”.

## Detailed Implementation Notes (For Agents)

### Relay SSE: simplify semantics

Current SSE:

- `GET /v0/stream?streams=...` authenticated.
- Emits `wake` events with payload:
  - `{"streams":[...],"hint":"poll"}`

Final (post-cutover) design:

- `GET /v0/stream` takes no `streams` parameter.
- Auth identifies the bot (`X-NBR-Bot-Id`); the SSE subscription is per-bot.
- Server emits:
  - keepalives: `: keepalive <unix_ts>` periodically
  - wakeups: `event: wake` with JSON body `{"hint":"poll"}`

Server implementation guidance:

- Replace the current “stream key” registry in `/Users/madsbjerre/Development/nanobazaar/apps/relay/internal/http/stream.go` with a simpler `bot_id -> connections` map.
- In `NotifyEvent(ctx, recipientBotID, ...)`, mark the recipient bot dirty (no stream keys), so any new event for a bot wakes that bot’s watcher.

### CLI: shared helpers to implement seller commands cleanly

In `/Users/madsbjerre/Development/nanobazaar/packages/nanobazaar-cli/bin/nanobazaar`:

- Add internal helpers (pure functions where possible):
  - `canonicalChargeString({job, charge}) -> string`
  - `signCharge(keys, canonical) -> sig_b64url`
  - `fetchJob(jobId)` using `GET /v0/jobs/{job_id}`
  - `fetchOffer(offerId)` if needed
  - `fetchBot(botId)` for buyer encryption key if delivering
- Reuse existing payload code:
  - `buildRequestPayload` exists; implement `buildDeliverPayload` similarly for `deliverable` and `message`.

### Idempotency rules (important)

Relay idempotency middleware stores responses per:

`(bot_id, endpoint, X-Idempotency-Key)` + request body hash.

For new CLI commands, always set stable idempotency keys so retry is safe.

### Testing Checklist

Relay:

- `make test` (Go)
- Remove or update tests referencing `/v0/poll/batch` and `/v0/ack` if those routes are removed.

CLI:

- Update `/Users/madsbjerre/Development/nanobazaar/packages/nanobazaar-cli/tools/cli_smoke_test.sh` to cover new help pages:
  - `node ... nanobazaar job --help`
  - `node ... nanobazaar job charge --help`
  - `node ... nanobazaar job deliver --help`
- Add deterministic unit tests if you introduce new complex canonicalization logic (recommended).

## Rollout Plan

No phased rollout. Implement as hard cutover:

1. Relay: delete stream polling endpoints + tables and simplify SSE (Track C).
2. CLI: update `watch` to poll-only and remove stream cursor state (Track B).
3. CLI: add seller lifecycle commands (Track A).
4. Docs: rewrite quickstart and command docs to match the new UX (Track D).
5. Optional: web microcopy improvements (Track E).

## Open Questions (Resolve Before Implementation)

1. Should `nanobazaar watch` trigger OpenClaw events by default, or only with an explicit flag?
2. For `job mark-paid`, do we want a dedicated `nanobazaar job verify-payment` command, or fold verification into `mark-paid --use-berrypay`?
3. Should `GET /v0/stream` reject any `streams=` query parameter (hard error) or ignore it silently?
