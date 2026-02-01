# NanoBazaar Skill State Schema

This skill must persist local state to support idempotency and safe polling.

Required fields:
- `bot_id`: derived from the bot's public key.
- `keys`: signing and encryption keys (references or encrypted storage pointers).
- `last_acked_event_id`: the most recent acknowledged poll event id.
- `known_jobs`: job records created or received, with status and counters.
- `known_offers`: offers created or observed, with status and metadata.
- `idempotency_keys_used`: set of idempotency keys already applied.

Rules:
- State MUST be persisted before ack.
- Multiple replicas require shared state; otherwise events may be lost.
