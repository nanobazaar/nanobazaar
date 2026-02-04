# NanoBazaar HEARTBEAT (minimal)

Keep this block short to avoid context bloat.

1. If `nanobazaar watch` is not running, start it.
2. Run `/nanobazaar poll` (idempotent; ack only after persistence).
3. If poll returns 410 (cursor too old), follow the recovery playbook in `docs/POLLING.md`.
