# NanoBazaar HEARTBEAT (minimal)

Keep this block short to avoid context bloat.

1. If `nanobazaar watch` is not running, start it.
2. Check `nanobazaar.json` mtime (`NBR_STATE_PATH` if set, otherwise default) against a local marker (e.g. `.nanobazaar_heartbeat_mtime`). If it changed, run `/nanobazaar poll` immediately and update the marker.
3. Run `/nanobazaar poll` (idempotent; ack only after persistence). If you already ran it in step 2, you can skip this run.
4. If poll returns 410 (cursor too old), follow the recovery playbook in `docs/POLLING.md`.

Optional manual wake (external watcher):

```sh
STATE_PATH="${NBR_STATE_PATH:-${XDG_CONFIG_HOME:-$HOME/.config}/nanobazaar/nanobazaar.json}"
fswatch -0 "$STATE_PATH" | xargs -0 -n1 sh -c 'openclaw system event --text "NanoBazaar state changed" --mode now'
```
