---
title: "feat(relay): Add Internal Error Logging"
type: feat
date: 2026-02-10
brainstorm: docs/brainstorms/2026-02-10-relay-error-logging-brainstorm.md
---

# feat(relay): Add Internal Error Logging

## Overview

Improve operability of the Relay by logging root-cause errors for server-side failures (5xx), without changing the API contract or response bodies.

We will keep the existing end-of-request `http_5xx ...` middleware log as a backstop, and add an additional log line at the source of internal errors where we still have the underlying `err`.

## Problem Statement / Motivation

Many handlers return `500` using `writeJSONError(..., "foo failed")` and drop the actual `err`. As a result, operators can see that a request failed (via `http_5xx`), but cannot see why it failed.

Goals:
- Make 5xx failures actionable from logs alone (request correlation + root error).
- Keep logs low-noise by skipping `context.Canceled` and `context.DeadlineExceeded`.
- Avoid adding dependencies or changing API responses.

Non-goals:
- Logging all 4xx.
- Changing status codes or error response JSON.
- Switching to a new logging framework or JSON logging in this iteration.

## Local Research Findings

Existing logging and middleware:
- `apps/relay/internal/http/router.go` uses `middleware.RequestID`, `middleware.RealIP`, and `middleware.Recoverer`.
- `apps/relay/internal/http/middleware.go` logs `http_5xx ... request_id=...` after requests with status >= 500.
- Most handlers use `apps/relay/internal/http/json.go` (`writeJSONError`) for generic error responses.

Where 500s are produced today:
- `apps/relay/internal/http/admin_handler.go` (many internal errors returned, few root causes logged)
- `apps/relay/internal/http/jobs.go`, `apps/relay/internal/http/offers.go`, `apps/relay/internal/http/bots.go`, `apps/relay/internal/http/poll.go`, `apps/relay/internal/http/payloads.go`
- `apps/relay/internal/auth/auth.go` returns 500 for idempotency failures but does not log the underlying `err` (only logs auth failures).

Institutional learnings:
- No `docs/solutions/` directory found in this repo at time of planning.

## Research Decision

Skip external research. This change is scoped to internal logging, uses existing `log.Printf` patterns, and does not introduce new dependencies or public behavior changes.

## SpecFlow Notes (Flows, Edge Cases, Gaps)

Key flows:
1. Request hits handler, underlying dependency fails, handler returns 500:
   - Handler emits `internal_error ... err=... request_id=...` (new).
   - Middleware emits `http_5xx ... request_id=...` (existing backstop).
2. Request hits auth middleware, idempotency read/write fails:
   - Auth middleware emits `internal_error ... err=... request_id=...` (new).
   - Handler not executed, but middleware still logs `http_5xx ...` (existing).
3. Request context is canceled or times out while handling:
   - We may still attempt to return 5xx, but we must not emit `internal_error` logs for `context.Canceled` / `context.DeadlineExceeded`.

Gaps to close in implementation:
- Define a stable `action` taxonomy for internal error logs (simple, consistent, not overly granular).
- Ensure logs do not leak sensitive information (never log request bodies, tokens, ciphertext).
- Ensure request correlation fields are present even when some values are missing (empty `request_id` should not break formatting).
- Logging tests must avoid global state conflicts (standard library `log` is global).

## Proposed Solution

### 1. Add a Small Internal Error Helper (HTTP Package)

Create a helper in `apps/relay/internal/http/` (package `httpapi`) to:
- Accept `*http.Request`, an `action` string, and `err`.
- If `err` is `context.Canceled` or `context.DeadlineExceeded`, do not log.
- Otherwise emit a consistent `log.Printf` line with:
  - `action`
  - `status=500` (or the status used)
  - `method`
  - `path` (prefer the URL path, optionally also the route pattern if cheap)
  - `bot_id` (from `X-NBR-Bot-Id` when present)
  - `request_id` (from `chi` request id middleware)
  - `remote_addr`
  - `err`

Keep response behavior identical by continuing to call `writeJSONError(w, status, message)` after logging.

Important: `internal/auth` must not import `internal/http` because `httpapi` already imports `auth` (it would create an import cycle). For auth middleware logging, either:
- Implement a tiny, duplicate helper inside `apps/relay/internal/auth/` with the same log format.
- Or introduce a new leaf package (for example `apps/relay/internal/reqlog`) that both `auth` and `httpapi` can import (it must not import either of them).

### 2. Update Main API Handlers (Non-Admin) First

Incrementally replace internal-error call sites that currently drop `err`, starting with non-admin endpoints:
- `apps/relay/internal/http/jobs.go`
- `apps/relay/internal/http/offers.go`
- `apps/relay/internal/http/bots.go`
- `apps/relay/internal/http/poll.go`
- `apps/relay/internal/http/payloads.go`
- `apps/relay/internal/http/stream.go` (where applicable)
- `apps/relay/internal/http/stats.go` (where applicable)

For each `500` path where an `err` is available, log via the helper before returning the existing JSON error.

### 3. Update Admin Endpoints

Apply the same pattern to admin handlers in `apps/relay/internal/http/admin_handler.go`. These are useful for ops and should benefit from root-cause logs, but can be done after main API endpoints.

### 4. Update Auth Middleware Internal Errors

For internal server errors in `apps/relay/internal/auth/auth.go` (idempotency lookup/insert failures):
- Log the underlying `err` with the same request correlation fields and an `action` label.
- Keep existing auth failure logs unchanged.
- Keep error response JSON unchanged.

Implementation note: because `auth` cannot depend on `httpapi`, prefer a local helper in `internal/auth` that:
- Uses `middleware.GetReqID(r.Context())` for `request_id` (add `github.com/go-chi/chi/v5/middleware` import to `auth`).
- Uses existing `canonicalPath(r)` for `path`.

## Technical Considerations

- Contract-first: no changes to `CONTRACT.md`, `OPENAPI.yaml`, `TEST_VECTORS.md`, and no response schema changes.
- Log duplication: expect two log lines for many 5xx responses (source + `http_5xx`). This is acceptable for now.
- Sensitive data: do not log request bodies or headers other than `X-NBR-Bot-Id` and `request_id`. Do not log admin tokens or payload ciphertext.
- Performance: `log.Printf` on error paths only; overhead should be negligible.

## Acceptance Criteria

- [x] For 5xx responses where an underlying `err` is available, the relay emits a source log line containing:
  - [x] `action=<...>`
  - [x] `request_id=<...>` when present
  - [x] `method=<...>` and `path=<...>`
  - [x] `bot_id=<...>` when present
  - [x] `remote_addr=<...>` when present
  - [x] `err=<...>`
- [x] For errors `errors.Is(err, context.Canceled)` or `errors.Is(err, context.DeadlineExceeded)`, the source log line is not emitted.
- [x] No API response codes or JSON bodies change.
- [x] Existing `http_5xx ...` middleware logging remains in place.
- [x] Auth middleware internal errors (idempotency lookup/insert failures) emit source error logs with the same correlation fields.

## Success Metrics

- Operators can identify the root error for a 5xx by searching logs for `request_id=<id>` and seeing an `internal_error` (or similarly named) source log line with `err=...`.
- Reduction in time-to-diagnose 5xx failures compared to current state where only generic messages are logged.

## Dependencies & Risks

Dependencies:
- None (use existing standard library `log`).

Risks:
- Increased log volume for 5xx-heavy periods due to duplicate logs (source + middleware).
- Logging tests may be flaky if they rely on global logger output without proper isolation.

Mitigations:
- Keep source logs limited to 5xx paths and skip canceled/deadline errors.
- Prefer an injectable logger or package-scoped logger variable in the helper to make tests deterministic.

## Testing Strategy

- Add unit tests for the helper to verify:
  - Logs are emitted for normal errors.
  - Logs are skipped for `context.Canceled` / `context.DeadlineExceeded`.
  - Key fields are present (at least `action`, `method`, `path`, `request_id` when set).
- Add one or two handler-level tests (existing handler tests already exist) to validate that a known internal error path produces the source log line.

Test isolation note: avoid relying on `log.SetOutput(...)` globally if packages run in parallel. Prefer making the helper use an injectable `*log.Logger` or a package-scoped logger variable/function that tests can override and restore.

## References

- Brainstorm: `docs/brainstorms/2026-02-10-relay-error-logging-brainstorm.md`
- Existing 5xx middleware log: `apps/relay/internal/http/middleware.go`
- Router middleware chain: `apps/relay/internal/http/router.go`
- JSON error helper: `apps/relay/internal/http/json.go`
- Auth middleware: `apps/relay/internal/auth/auth.go`
