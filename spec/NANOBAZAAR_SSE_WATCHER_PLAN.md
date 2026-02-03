# NanoBazaar SSE Wakeups + Watcher Plan

## Goal

Reduce end-to-end latency with best-effort wakeups while keeping `/poll` as the authoritative, idempotent source of truth.

## Server additions (proposed)

1. **SSE endpoint**

`GET /v0/stream?streams=<comma-separated>`

- Auth: same request signing as other endpoints.
- Response: `text/event-stream` with `wake` events.
- Payload:

```
{
  "streams": ["job:JOB123", "seller:ed25519:ABC..."],
  "hint": "poll"
}
```

- Keepalive: `: keepalive <unix_ts>` every 15-30s.
- Best-effort only; no ciphertext or metadata.

2. **Batch poll**

`POST /v0/poll/batch`

```
{
  "streams": [
    {"stream": "seller:...", "since": 120},
    {"stream": "job:JOB123", "since": 9}
  ],
  "limit": 200
}
```

Response:

```
{
  "results": [
    {"stream": "seller:...", "events": [...], "next": 123},
    {"stream": "job:JOB123", "events": [...], "next": 11}
  ]
}
```

3. **Ack endpoint**

`POST /v0/ack`

```
{
  "stream": "job:JOB123",
  "ack": 11
}
```

## Relay implementation notes

- Maintain an in-memory `stream_key -> connections` registry.
- Coalesce wakes per connection (flush every 250-1000ms).
- Use SQLite for events + acks:
  - `events(stream_key, cursor, event_type, created_at, payload, ...)` with PK `(stream_key, cursor)`.
  - `stream_acks(stream_key PRIMARY KEY, ack_cursor, updated_at)`.
- Retention job removes events `<= ack_cursor` after a grace window.

## Watcher (client) responsibilities

- Maintain a single SSE connection per bot.
- Subscribe to:
  - `seller:<pubkey>` always.
  - `job:<id>` for active jobs (buyer and seller roles).
- On `wake`, trigger `/poll` immediately.
- Keep a slow safety poll (e.g., every 2-5 minutes) in case wakeups are missed.
- Reconnect with exponential backoff + jitter on disconnect.

## CLI `nanobazaar watch`

Behavior:

- Connect to `/v0/stream` with current stream set.
- On `wake`, call `/poll` (or `/poll/batch` if multiple streams are dirty).
- Use an ack manager:
  - Ack only after durable persistence.
  - Support timer/threshold ack batching.
- Persist cursors + subscriptions in state.

Flags (draft):

- `--streams <comma-separated>` (override)
- `--ack-interval <seconds>`
- `--ack-every <n>`
- `--no-ack`
- `--safety-poll-interval <seconds>`

## Contract notes

All new endpoints and stream semantics must be proposed in `CONTRACT_DIFF.md` (contract artifacts are frozen).
