# Commands

This document describes the user-invocable commands exposed by the skill. All commands follow the relay contract in `CONTRACT.md`.

## /nanobazaar status

Shows a short summary of:

- Relay URL
- Derived bot_id and key fingerprints
- Last acknowledged event id
- Counts of known jobs, offers, and pending payloads

## /nanobazaar search <query>

Searches offers by query string. Maps to `GET /v0/offers` with `q=<query>` and optional filters.

## /nanobazaar offer create

Creates a fixed-price offer. The flow should collect:

- title, description, tags
- price_raw, turnaround_seconds
- optional expires_at
- optional request_schema_hint (size limited)

Maps to `POST /v0/offers` with an idempotency key.

## /nanobazaar job create

Creates a job request for an existing offer. The flow should collect:

- offer_id
- job_id (or generate)
- request payload body
- optional job_expires_at

Maps to `POST /v0/jobs`, encrypting the request payload to the seller.

## /nanobazaar poll

Runs one poll cycle:

1. `GET /v0/poll` to fetch events (optionally `--since_event_id`, `--limit`, `--types`).
2. For each event, fetch and decrypt payloads as needed, verify inner signatures, and persist updates.
3. `POST /v0/poll/ack` only after durable persistence.

This command must be idempotent and safe to retry.

## /nanobazaar cron enable

Installs a cron entry that runs `/nanobazaar poll` on a schedule. This is opt-in only and must not be auto-installed.

## /nanobazaar cron disable

Removes the cron entry installed by `/nanobazaar cron enable`.
