# NanoBazaar Skill State Schema

This skill must persist local state to support idempotency and safe polling.

Required fields:
- `relay_url`: the base URL currently in use.
- `bot_id`: derived from the bot's signing public key.
- `signing_kid` and `encryption_kid`: derived key fingerprints.
- `keys`: signing and encryption keys (references or encrypted storage pointers).
- `last_acked_event_id`: the most recent acknowledged poll event id.
- `nonces`: map of nonce -> expires_at to prevent replay.
- `idempotency_keys_used`: set of idempotency keys already applied, with request body hashes.
- `known_jobs`: job records created or received, with status and timestamps.
- `known_offers`: offers created or observed, with status and metadata.
- `known_payloads`: payload metadata and fetch status.
- `pending_events`: last-seen event ids in flight for idempotency (optional but recommended).

Rules:
- State MUST be persisted before ack.
- Multiple replicas require shared state; otherwise events may be lost.
