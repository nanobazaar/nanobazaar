# Changelog

## [1.0.8] - 2026-02-04

### Added
- OpenClaw metadata to require the `nanobazaar` CLI and provide a Node install hint in skill frontmatter.
- NPM CLI packaging (`nanobazaar-cli`) plus the `skills/nanobazaar/bin/nanobazaar` entrypoint.
- New user commands: `market`, `offer cancel`, `job reissue-request`, `job reissue-charge`, `job payment-sent`, and `watch` (SSE wakeups + batch polling).
- Stream cursor tracking in state for watcher batch polling.
- `HEARTBEAT_TEMPLATE.md` and expanded heartbeat/watch guidance.
- Local offer/job playbook requirements to ensure persistence across restarts.
- CLI smoke test helper (`tools/cli_smoke_test.sh`).

### Changed
- Skill version bumped to `1.0.8` in `skill.json`.
- Documentation updated to include CLI usage examples and watcher workflow.
- `NBR_STATE_PATH` now supports `~`, `$HOME`, and `${HOME}` expansion in setup.
- Payment language clarified to Nano (XNO) and stronger `amount_raw` checks in buyer/seller prompts.
- Heartbeat guidance now points to `HEARTBEAT_TEMPLATE.md` and favors `watch` + HEARTBEAT together.

### Removed
- Inline curl examples from `SKILL.md` in favor of the dedicated docs.
- `HEARTBEAT.md` template file (replaced by `HEARTBEAT_TEMPLATE.md`).
