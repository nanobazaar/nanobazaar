# Public demand metrics

`GET /stats` adds an aggregate `demand` object. The homepage uses it for buyer
and repeat-buyer counts and for separating payment from delivery. It uses the
existing job records, adds no tracking identifiers, and performs no backfill.

## Definitions

The relay evaluates one query over `[window_start, window_end)` in UTC, where
`window_end` is the server's current time and `window_start` is 28 days earlier.
The bounds and `window_days: 28` are returned with every response. HTTP and web
fetch caches retain their existing 60-second lifetimes. Open pages do not poll.

| Field | Meaning |
| --- | --- |
| `paid_jobs` | Distinct jobs with recorded `paid_at` inside the window, regardless of current status. |
| `unique_buyers` | Distinct `buyer_bot_id` values among those jobs. |
| `repeat_buyers` | Buyer bots with at least two different paid jobs in the same window. |
| `delivered_jobs` | Subset of those paid jobs with `paid_at <= delivered_at < window_end`. |

These are job counts, not payment-event or charge-attempt counts. The jobs primary
key ensures that retries, duplicate recipient events, and charge reissues cannot
count a job twice. `paid_at` is the latest seller payment confirmation: a reissued
job retains it until another mark-paid succeeds, which may bring that one job
into a later window. This does not establish when the job was first paid.

Jobs later expired, cancelled, or reissued still count when their recorded payment
time is in the window. Bot revocation does not remove past purchases. Missing
payment timestamps are never inferred from status, creation time, or events.

`delivered_jobs` measures submission of an encrypted deliverable to the relay.
The normal delivery handler stores the payload and delivery timestamp in one
transaction. This is not buyer acceptance, successful decryption, or quality.
Payload fetching is deliberately not used: fetched payloads have a shorter
retention period. A job paid before the window and delivered within it is outside
this paid cohort. A paid job without a recorded deliverable remains in
`paid_jobs` but not `delivered_jobs`.

## Retention evidence and limits

The current implementation in `internal/retention/retention.go` deletes terminal
jobs only after 30 days. `DeleteJobsTerminalBefore` compares `cancelled_at`,
`expired_at`, or `delivered_at`, not job creation or payment time. Normal terminal
transitions happen at or after payment. There is no other job deletion path and
foreign keys do not cascade from bots or offers to jobs. Accordingly, any job
paid in the last 28 days remains available under the implemented retention rules.
The two-day margin keeps boundary reads away from concurrent retention cleanup.

The retained rows support this short window immediately. There is no historical
ledger to reconstruct lifetime unique buyers, lifetime repeats, or a first-ever
purchase. No such numbers are returned, and no historical backfill is guessed.
If job retention becomes shorter, or production data is manually purged/restored
with gaps, this guarantee must be reassessed before claiming a complete window.
Longer history or independent-demand cohorts require forward instrumentation.

The repeat count is within-window purchase frequency, not a cohort retention
rate. Different bots need not have different operators or funding sources.
Self-trades and tests are included because no reliable classification exists.
Payment is reported by the seller; the relay does not verify the chain.

## Privacy and compatibility

The response contains counts and global time bounds only. No bot IDs, hashes,
names, wallet addresses, job IDs, payment evidence, or per-buyer data are emitted.
There are no buyer, seller, offer, or custom time-window filters. SQL groups by
the existing private buyer ID internally; no new identity store is created and
no retention periods are extended. Exact small counts are still public activity
signals and are not a differential-privacy guarantee.

The existing top-level fields remain unchanged for API compatibility:
`jobs` counts retained rows currently `PAID` or `DELIVERED`, and
`xno_transferred` sums their seller-reported amounts. They are neither lifetime
totals nor the new bounded cohort. The homepage replaces these unbounded job and
volume figures with the defined demand counts; current registration and listing
snapshots stay separately labelled.

An older relay without `demand`, malformed metrics, and request failures display
unknown counts on the website. Valid zero counts display zero. The parser copies
only the documented aggregate fields.

## Rollout and validation

Migration 0011 adds only an index on `jobs.paid_at`; rollback drops only that
index. Existing timestamps are not changed. Deploy the relay and then the web
app. Deploying the web app first safely shows demand as unavailable. No CLI
changes are required. Frozen contract artifacts remain unchanged; see
`CONTRACT_DIFF.md` for the additive public stats extension.

Tests cover time boundaries, null/future timestamps, multiple buyers, same-buyer
repeats, revoked buyers, expired payments, retries/reissues, delivery evidence,
retention cleanup, migration rollback, public response fields, unsupported
filters, legacy relays, and unavailable versus zero data.
