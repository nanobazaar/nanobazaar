---
date: 2026-02-10
topic: relay-error-logging
---

# Relay Error Logging

## What We're Building
Add actionable error logging for server-side failures in the Relay app, without changing the API contract or response bodies.

Today we often return `500` with a generic message (via `writeJSONError`) but do not log the underlying `err` at the point of failure, so operators only see end-of-request `http_5xx ...` logs without a cause (see `apps/relay/internal/http/middleware.go`).

We will add a consistent, request-correlated log line at the *source* of internal failures (places where we currently return `500` and still have the `err` in hand). These logs should include: `request_id`, `method`, `path`, `bot_id` (when present), `remote_addr`, a short `action` label, and `err`.

Scope is intentionally limited to server-side failures: panics (already logged by `chi` recoverer) and `5xx` responses. We will not add general logging for `4xx` responses.

To reduce noise, we will *not* log errors that are clearly due to client disconnects/timeouts (for example `context.Canceled` / `context.DeadlineExceeded`), even if they happen to surface while handling a request.

## Why This Approach
We considered three options:

1. Log at the failure site (recommended).
2. Capture an error in request scope and log once in middleware.
3. Log deeper in store/DB boundaries and rely on middleware for request context.

We chose (1) because it is the smallest conceptual change and fits existing patterns: the only place we reliably have the root `err` is where we decide to return `500`. This also allows incremental adoption: we can update the highest-value `500` call sites first without building new plumbing.

We will keep the existing `http_5xx ...` middleware log as a backstop, accepting duplicate log lines per `500` for now. The middleware log is still useful to catch any `5xx` that slips through without a source log.

## Key Decisions
- Scope: log only server-side failures (`5xx` and panics), not routine `4xx`.
- Format: keep `log.Printf` with `key=value` fields to stdout/stderr (no JSON logger migration).
- Noise control: skip logging for `context.Canceled` / `context.DeadlineExceeded`.
- Mechanism: log at the source of the `500` where the underlying `err` is available.
- Backstop: keep existing `http_5xx ...` middleware logging even if it duplicates source logs.

## Open Questions
- Do we later want to deduplicate to one log line per request (source vs middleware)?
- Should error logging be configurable (env flag) for more/less verbosity in production?
- Do we want optional stack traces for non-panic errors, or keep logs to `err` only?
- Should we extend beyond `5xx` to a small set of important `4xx` cases (and which ones), given auth failures and rate limits are already logged?

## Next Steps
-> Run `/prompts:workflows-plan` to define the helper interface, enumerate/triage `500` call sites, and decide on a lightweight testing strategy for "logs emitted" and "canceled contexts skipped".
