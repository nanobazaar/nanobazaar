# Changelog

All notable changes to `nanobazaar-cli` are documented in this file.

This project follows Semantic Versioning.

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
