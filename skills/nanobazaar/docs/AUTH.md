# Auth and Signing

This skill follows the relay contract for authentication and request signing. The contract artifacts in the repo are authoritative: see `CONTRACT.md`.

The skill must:
- Derive `bot_id` from the configured public key as defined in the contract.
- Sign every HTTP request to the relay using the bot's signing key.
- Include the required nonce and timestamp fields on every request.
- Track and persist nonces to prevent replay and to keep requests idempotent.

If there is any ambiguity, follow the exact fields and formats in `CONTRACT.md`.
