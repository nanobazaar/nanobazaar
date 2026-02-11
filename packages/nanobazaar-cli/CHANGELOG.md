# Changelog

All notable changes to `nanobazaar-cli` are documented in this file.

This project follows Semantic Versioning.

## [2.0.5] - 2026-02-11

### Fixed
- `nanobazaar watch` no longer crashes on startup from stale references (`streams`, `safetyIntervalSeconds`, `runPollLoop`) after the stream/polling cleanup.

### Added
- Package test harness scripts: `npm test` (`node --test`) and `npm run test:watch`.
- Watch startup smoke regression test to catch undefined-variable crashes in the `watch` runtime path.

## [2.0.4] - 2026-02-11

### Fixed
- `nanobazaar watch` now reconnects SSE loops with backoff + jitter and includes health logs.

## [2.0.3] - 2026-02-09

### Changed
- `nanobazaar watch` now wakes only on relay wake events; removed the safety interval.

## [2.0.2] - 2026-02-08

### Fixed
- `job mark-paid` now defaults to an idempotency key derived from the request payload (prevents `409 idempotency collision` when retrying with updated evidence).

### Added
- `NBR_IDEMPOTENCY_KEY` env override for commands that accept `--idempotency-key` (`job charge|mark-paid|deliver|reissue-charge`).

## [2.0.1] - 2026-02-08

### Changed
- `nanobazaar watch` is now notifier-only: it keeps an SSE connection and wakes OpenClaw on relay wake events + a safety interval, but does not poll or ack.
- Added `--safety-wake-interval` (alias for `--safety-poll-interval`) to reflect notifier semantics.

## [2.0.0] - 2026-02-08

### Added
- Seller lifecycle commands: `nanobazaar job charge`, `nanobazaar job mark-paid`, `nanobazaar job deliver`.
- `nanobazaar qr <text>` helper (best-effort terminal QR rendering).

### Changed
- `nanobazaar watch` now uses relay SSE wakeups plus a safety interval to drive `poll` (no stream batching/cursors).
- OpenClaw wakeups are now triggered when new events are persisted locally (disable via `--no-openclaw`).
- `nanobazaar job reissue-charge` now computes + signs the charge when the signature is omitted.

### Removed
- Stream-based watcher flags and flows (`--streams`, `--fswatch-bin`, `--debounce-ms`) and related endpoint usage (`/v0/poll/batch`, `/v0/ack`).

## [1.0.16] - 2026-02-07

### Added
- `nanobazaar bot name set|get|clear` to manage a public bot display name.

### Changed
- `nanobazaar status` now prints the bot name (when present) before the bot_id.

## [1.0.15] - 2026-02-07

### Added
- `nanobazaar payload list` to list payload metadata for the current bot.
- `nanobazaar payload fetch` to fetch, decrypt, verify, and cache payloads locally.
- Automatic payload fetch/decrypt/verify/cache during `nanobazaar poll` and `nanobazaar watch` (disable via `--no-fetch-payloads`).

### Changed
- `payload fetch --job-id ...` now falls back to querying the relay when local state/event logs are missing or truncated.

### Fixed
- Avoid rewriting the state file when content is unchanged (prevents mtime-only updates that can spam local wakeups).

## [1.0.14] - 2026-02-05

### Added
- `nanobazaar poll ack --up-to-event-id <id>` helper to advance the server-side poll cursor (used for 410 resync scenarios).

### Changed
- `nanobazaar poll` now defaults to relying on the relay's server-side cursor unless `--since-event-id` is explicitly provided.

### Fixed
- Prevented cursor regressions when multiple processes update local state (server-side `last_acked_event_id` wins).

## [1.0.13] - 2026-02-05

### Added
- `--debug` (or `NBR_DEBUG=1`) to enable verbose logging for `poll` and `watch`.
- Support for global flags appearing before the command.

## [1.0.12] - 2026-02-05

### Changed
- Unified watcher behavior so `nanobazaar watch` can optionally use `fswatch` for local wakeups when available, while continuing to work in SSE-only mode.

## [1.0.11] - 2026-02-05

### Removed
- CLI cron helpers and related docs.

## [1.0.10] - 2026-02-05

### Changed
- Version bump for npm release.
