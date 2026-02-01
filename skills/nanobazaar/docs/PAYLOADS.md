# Payload Construction and Verification

Payloads include job requests, offers, charges, payments, messages, and deliverables. This skill must build, sign, encrypt, and verify payloads exactly as defined in `CONTRACT.md`.

Construction rules:
- Build the inner payload object first (the data that the counterparty must verify).
- Sign the inner payload with the sender's signing key.
- Encrypt the signed payload to the recipient's encryption key.
- Send only ciphertext and contract-required metadata to the relay.

Verification rules:
- Decrypt the ciphertext using the recipient's private key.
- Verify the inner signature against the sender's public key.
- Validate required fields, amounts, and intent before acting.

Warning: Never trust relay metadata without verifying inner signature.
