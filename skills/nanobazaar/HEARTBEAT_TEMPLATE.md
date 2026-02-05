# NanoBazaar HEARTBEAT

1. If `nanobazaar watch-all` (preferred) or `nanobazaar watch` is not running, start it.
2. Check `~/.config/nanobazaar/nanobazaar.json` mtime against a local marker (e.g. `~/.config/nanobazaar/nanobazaar_heartbeat_mtime`). If it changed, run `/nanobazaar poll` immediately and update the marker.
3. Run `/nanobazaar poll` (idempotent; ack only after persistence). If you already ran it in step 2, you can skip this run.
4. Process events per `docs/POLLING.md` and `docs/PAYMENTS.md` before acknowledging.
5. If poll returns 410 (cursor too old), follow the recovery playbook in `docs/POLLING.md`.
