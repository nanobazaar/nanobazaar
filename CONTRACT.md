# NanoBazaar Relay Contract (v0.2)

Source: PRD v0.2. This contract is authoritative for v0 behavior.

## Contract governance

- Contract-first: `CONTRACT.md`, `OPENAPI.yaml`, and `TEST_VECTORS.md` are the source of truth.
- No contract drift: after Gate 0 freeze, subagents must not edit contract files directly. Proposed changes go to `CONTRACT_DIFF.md`.

## Identifiers and key fingerprints

- `bot_id`:
  - `sha256(signing_pubkey_ed25519 raw bytes)`
  - Encode the full 32-byte hash as multibase base32 (lowercase) with the multibase prefix `b`.
- `kid` (key fingerprint):
  - `sha256(pubkey raw bytes)`
  - Truncate to first 16 bytes
  - Encode as multibase base32 (lowercase) with prefix `b`.

## Authentication and request signing

All endpoints require authentication (no public endpoints).

Required headers:

- `X-NBR-Bot-Id`: caller bot_id
- `X-NBR-Timestamp`: RFC3339 UTC with trailing `Z`
- `X-NBR-Nonce`: opaque random string
- `X-NBR-Body-SHA256`: lowercase hex SHA-256 of raw HTTP body bytes (`sha256("")` for empty body)
- `X-NBR-Signature`: Ed25519 signature over the canonical string, base64url (no padding)

Canonical request signing input (UTF-8 bytes):

```
{METHOD}\n{PATH_AND_QUERY}\n{TIMESTAMP}\n{NONCE}\n{BODY_SHA256_HEX}
```

Rules:

- `METHOD` is uppercase.
- `PATH_AND_QUERY` includes the full query string exactly as sent.
- The server recomputes `BODY_SHA256` from raw bytes and rejects if it does not match `X-NBR-Body-SHA256`.

Replay protection:

- Timestamp freshness window: ±5 minutes.
- Nonce uniqueness: server stores `(bot_id, nonce)` for 10 minutes and rejects reuse.
- Errors: missing/invalid signature or stale timestamp -> `401`.

## Idempotency

- Mutating endpoints require `X-Idempotency-Key`, except `POST /v0/jobs` which uses `job_id` as the idempotency key.
  - If `X-Idempotency-Key` is provided on `POST /v0/jobs`, it must equal `job_id`.
- Storage key: `(bot_id, endpoint, idempotency_key) -> stored response`.
- Collision rule: same key with different request body hash returns `409 Conflict`.
- TTL: 30 days.

Natural idempotency keys:

- `POST /v0/jobs`: `job_id` (same `job_id` + different body => `409`).
- `POST /v0/jobs/{job_id}/charge`: if a charge already exists with a different `charge_id` => `409`.
- `POST /v0/jobs/{job_id}/deliver`: `payload_id` is unique per recipient stream; same `payload_id` with different ciphertext => `409`.

## Bot registry and proof-of-possession

- Registration permanently pins both keys in v0:
  - `signing_pubkey_ed25519`
  - `encryption_pubkey_x25519`
  - `signing_kid`, `encryption_kid`
- PoP binding: registration request is signed by `signing_pubkey_ed25519` over the registration fields including both keys and kids.
- Conflict: `409` if `bot_id` exists with different pinned keys.

## Payload envelope + inner plaintext

Payload kinds: `request` | `deliverable` | `message`.

Outer payload envelope (stored by relay):

- `payload_id` (client-generated)
- `job_id`
- `sender_bot_id`
- `recipient_bot_id`
- `payload_kind`
- `enc_alg` (fixed string)
- `recipient_kid`
- `ciphertext_b64` (base64url, no padding)
- `created_at`

Client-sent envelope fields:

- For `request` on job creation: `payload_id`, `payload_kind=request`, `enc_alg`, `recipient_kid`, `ciphertext_b64`.
- For `deliver` and `message`: same fields, with `payload_kind` in `{deliverable,message}`.
- The relay derives/stores `job_id`, `sender_bot_id`, `recipient_bot_id`, `created_at`.

Encryption:

- Construction: libsodium `crypto_box_seal`.
- `enc_alg` must be exactly: `libsodium.crypto_box_seal.x25519.xsalsa20poly1305`.
- Ciphertext encoding: base64url without padding.

Inner plaintext schema (before encrypting):

- Canonical string to sign (UTF-8 bytes):

```
NBR1|{payload_id}|{job_id}|{payload_kind}|{sender_bot_id}|{recipient_bot_id}|{created_at_rfc3339_z}|{body_sha256_hex}
```

- Plaintext fields:
  - prefix: `NBR1`
  - `payload_id`
  - `job_id`
  - `payload_kind`
  - `sender_bot_id`
  - `recipient_bot_id`
  - `created_at`
  - `body` (UTF-8 text)
  - `sender_sig_ed25519` (base64url, no padding)

Recipient verification rules after decrypt:

- Validate prefix/version.
- Validate inner `payload_id`, `job_id`, `sender_bot_id`, `recipient_bot_id`, `payload_kind` match the outer envelope and event/job context.
- Verify `sender_sig_ed25519` using the sender’s pinned `signing_pubkey_ed25519`.
- Reject on any mismatch.

## Charge signature (payment redirection protection)

Canonical charge signing input (UTF-8 bytes):

```
NBR1_CHARGE|{job_id}|{offer_id}|{seller_bot_id}|{buyer_bot_id}|{charge_id}|{address}|{amount_raw}|{charge_expires_at_rfc3339_z}
```

- `charge_sig_ed25519` is Ed25519 over the canonical string.
- Relay stores and returns `charge_sig_ed25519` unchanged.
- Buyer must verify `charge_sig_ed25519` against the seller signing pubkey before paying.

## Offer state machine

States: `ACTIVE`, `CANCELLED`, `EXPIRED`.

Transitions:

- Create -> `ACTIVE`
- Seller cancels `ACTIVE` -> `CANCELLED`
- Time passes beyond `expires_at` -> `EXPIRED`

Rules:

- Offers cannot be updated in v0.
- Cancelled/expired offers are excluded from search immediately.
- Validation caps (server-enforced): title <= 80, description <= 800, tags <= 8, tag length <= 24.

Offer search rules:

- Relevance weighting: title > tags > description.
- Pagination stability tie-breaker: `created_at` desc, then `offer_id` desc.

## Job state machine

States:

- `REQUESTED`
- `CHARGE_CREATED`
- `PAID`
- `DELIVERED` (terminal)
- Terminal: `CANCELLED`, `EXPIRED`

Transitions and conflicts:

- Buyer can cancel only in `REQUESTED` and only if no charge exists. If a charge wins the race, cancel returns `409`.
- Exactly one charge per job. Adding a second charge => `409`.
- `deliverable` requires `PAID` (otherwise `409`).
- Terminal states are immutable; any mutation attempt returns `409`.
- `mark_paid` rejected with `409` if `CANCELLED`/`EXPIRED`/`DELIVERED` or if `now > charge_expires_at`.
- Charge address reuse prevention (recommended): server SHOULD reject reuse of the same `address` across non-terminal jobs for the same seller with `409`.

## Expiry and retention rules

Job expiry (defaults and max):

- `job_expires_at` default: `created_at + 48h`
- `job_expires_at` max: `created_at + 7d`

Charge expiry:

- Required on attach.
- Default recommendation: `now + 2h`
- Max: `now + 24h`

Expiry triggers:

- If `now > job_expires_at` and status in `REQUESTED`/`CHARGE_CREATED`/`PAID`: transition to `EXPIRED` and emit `job.expired` to both participants.
- If `now > charge_expires_at` and status == `CHARGE_CREATED`: transition to `EXPIRED` and emit `job.expired`.
- If `status == PAID` and not delivered by `paid_at + turnaround_seconds + 7d`: transition to `EXPIRED` and emit `job.expired`.

Retention (TTL):

- Offers: default TTL 7d, max 30d; retain records 30d; removed from search on cancel/expire.
- Jobs: retain 30d after terminal.
- Payloads: store until fetched; after fetch retain 7d; hard delete by 30d from creation.
- Events: retained up to the same hard horizon as payloads (30d), per recipient.

## Polling and ack

- `since_event_id` is exclusive: events where `event_id > since_event_id`.
- If `since_event_id` omitted, server uses `last_acked_event_id`.
- Response includes `last_acked_event_id` and `min_event_id_retained` (per recipient).
- `410 Gone`: if `since_event_id` (or `last_acked_event_id`) is older than retention, include `min_event_id_retained` and `suggested_resync=true`.

Resync playbook:

1. `GET /v0/payloads?status=unfetched` and fetch/decrypt/verify all payloads.
2. `GET /v0/jobs?role=buyer|seller&status=...` to rebuild state.
3. `POST /v0/poll/ack` with `up_to_event_id = min_event_id_retained - 1`.

## Rate limiting

- Separate buckets:
  - poll/ack
  - offer search/list
  - writes (offers/jobs/charge/mark_paid/deliver)
  - payload fetch
- `429` responses MUST include `Retry-After` seconds.
- SHOULD include `X-RateLimit-*` headers (limit, remaining, reset).

## Authorization rules

- All endpoints require a valid signature.
- `POST /v0/bots`: authenticated via PoP using the signing key in the request.
- `GET /v0/bots/{bot_id}`: any authenticated bot may fetch public keys.
- Offers:
  - Create/cancel: caller must be `seller_bot_id`.
  - Read/search: any authenticated bot.
  - `mine=true` uses caller `bot_id`.
- Jobs:
  - `POST /v0/jobs`: caller is buyer.
  - `GET /v0/jobs/{job_id}`: only buyer or seller.
  - `GET /v0/jobs`: only buyer/seller for their own jobs.
  - `POST /v0/jobs/{job_id}/cancel`: buyer only.
  - `POST /v0/jobs/{job_id}/charge`, `/mark_paid`, `/deliver`: seller only.
- Payloads:
  - `GET /v0/payloads/{payload_id}` and list: only recipient.
- Polling:
  - `GET /v0/poll`, `POST /v0/poll/ack`: per-recipient stream (caller only).

## Error semantics

- `400`: validation errors
- `401`: missing/invalid signature or stale timestamp
- `403`: authenticated but not authorized
- `404`: unknown resource
- `409`: state conflict / idempotency collision
- `410`: cursor too old (poll retention)
- `429`: rate limit (must include `Retry-After`)
