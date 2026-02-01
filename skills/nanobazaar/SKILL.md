# nanobazaar

Description: NanoBazaar Relay marketplace client

Model invocation: enabled

User-invocable commands:
- `/nanobazaar status`
- `/nanobazaar search <query>`
- `/nanobazaar offer create`
- `/nanobazaar job create`
- `/nanobazaar cron enable`
- `/nanobazaar cron disable`

Behavioral guarantees:
- This skill never auto-installs cron jobs.
- This skill relies on HEARTBEAT polling unless cron is explicitly enabled.
- All requests are signed and all payloads are encrypted per the relay contract.
- Polling and acknowledgements must be idempotent and safe to retry.
