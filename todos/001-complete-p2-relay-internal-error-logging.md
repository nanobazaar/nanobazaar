---
status: complete
priority: p2
issue_id: "001"
tags: [relay, logging]
dependencies: []
---

# Add Internal Error Logging For Relay 5xx

## Problem Statement

Relay handlers often return `500` with a generic JSON error message but do not log the underlying `err`, making production failures hard to diagnose from logs alone.

## Findings

- `apps/relay/internal/http/middleware.go` logs `http_5xx ...` at end-of-request, but it does not include the root `err`.
- Many handlers call `writeJSONError(w, http.StatusInternalServerError, "...failed")` after `err != nil` without logging that `err`.
- `apps/relay/internal/auth/auth.go` returns `500` for idempotency lookup/insert errors without logging the underlying `err`.

## Proposed Solutions

### Option 1: Log At The Source (Recommended)

**Approach:** Add a small helper and call it right before returning `500`, where the underlying `err` is still available.

**Pros:**
- Root cause captured with request correlation fields.
- Incremental adoption and low conceptual overhead.

**Cons:**
- Touches many call sites.
- Duplicate 5xx log lines (source + existing middleware backstop).

**Effort:** Medium

**Risk:** Low

### Option 2: Capture Error In Request Scope And Log Once In Middleware

**Approach:** Add request-scoped error capture and log once at end-of-request.

**Pros:**
- Single log line per request.

**Cons:**
- More plumbing; easy to miss setting the error.

**Effort:** Medium-high

**Risk:** Medium

## Recommended Action

Implement Option 1. Log only 5xx source errors, keep existing `http_5xx` middleware log as a backstop, and skip logging for `context.Canceled` / `context.DeadlineExceeded` to reduce noise. Do not change API responses.

## Acceptance Criteria

- [x] Source log line emitted for 5xx paths where `err` is available, including `action`, `request_id`, `method`, `path`, `bot_id`, `remote_addr`, `err`.
- [x] Source log line is skipped for `context.Canceled` / `context.DeadlineExceeded`.
- [x] No API response codes or JSON bodies change.
- [x] Auth idempotency internal errors emit source logs with the same correlation fields.
- [x] Tests cover "logs emitted" and "canceled/deadline skipped".

## Work Log

### 2026-02-10 - Execution Started

**By:** Codex

**Actions:**
- Created implementation plan in `docs/plans/2026-02-10-feat-relay-internal-error-logging-plan.md`
- Started implementing work on branch `feat/relay-internal-error-logging`

**Learnings:**
- `internal/auth` cannot import `internal/http` helpers due to an import cycle; logging helper must live in `auth` or a new leaf package.

### 2026-02-10 - Completed

**By:** Codex

**Actions:**
- Added `internal_error` logging helpers in `apps/relay/internal/http/internal_error.go` and `apps/relay/internal/auth/auth.go`
- Updated main HTTP handlers and admin handlers to log root-cause `err` on 5xx paths while keeping responses unchanged
- Added tests for log emission and cancellation/deadline skipping
- Ran `make fmt`, `make test`, `make lint`
