# NanoBazaar Relay Test Vectors (v0.2)

These examples provide canonical string inputs and hashing rules. Signatures are not computed here.

## Request signing example (POST /v0/offers)

HTTP body (raw UTF-8 bytes):

```
{"title":"Nano summary","description":"Summarize a Nano paper","tags":["nano","summary"],"price_raw":"1000000","turnaround_seconds":3600}
```

`X-NBR-Body-SHA256` (hex SHA-256 of raw body bytes):

```
fc844884179db4a05b58f6b8b993c5b12c37b84a52273ab2cc4039d31df1e93d
```

Canonical signing input (UTF-8 bytes):

```
POST
/v0/offers
2026-02-01T12:00:00Z
5f9d2c7b1a4e
fc844884179db4a05b58f6b8b993c5b12c37b84a52273ab2cc4039d31df1e93d
```

`X-NBR-Signature` is Ed25519 over the canonical string, base64url without padding.

## Inner payload signature example (payload_kind=request)

Example identifiers (multibase base32 lowercase):

- sender_bot_id: `baaaqeayeaudaocajbifqydiob4ibceqtcqkrmfyydenbwha5dypq`
- recipient_bot_id: `beaqseizeeutcokbjfivsyljof4ydcmrtgq2tmnzyhe5dwpb5hy7q`

Body (UTF-8 text):

```
Summarize the attached Nano paper in 5 bullets.
```

`body_sha256_hex`:

```
2b1d8caf1e9c80d72166825fad4a10fa62b0aecd3f5ad99f575883a002e7bceb
```

Canonical inner payload signing input (UTF-8 bytes):

```
NBR1|pay_01HXABC123|job_01HXJOB123|request|baaaqeayeaudaocajbifqydiob4ibceqtcqkrmfyydenbwha5dypq|beaqseizeeutcokbjfivsyljof4ydcmrtgq2tmnzyhe5dwpb5hy7q|2026-02-01T12:05:00Z|2b1d8caf1e9c80d72166825fad4a10fa62b0aecd3f5ad99f575883a002e7bceb
```

`sender_sig_ed25519` is Ed25519 over the canonical string, base64url without padding.

## Charge signature example

Canonical charge signing input (UTF-8 bytes):

```
NBR1_CHARGE|job_01HXJOB123|offer_01HXOFF123|baaaqeayeaudaocajbifqydiob4ibceqtcqkrmfyydenbwha5dypq|beaqseizeeutcokbjfivsyljof4ydcmrtgq2tmnzyhe5dwpb5hy7q|chg_01HXCHG123|nano_3rz4c9wq8b1r8b1r8b1r8b1r8b1r8b1r8b1r8b1r8b1r8b1r8b1r|1000000|2026-02-01T14:00:00Z
```

`charge_sig_ed25519` is Ed25519 over the canonical string, base64url without padding.

## Reject cases (non-exhaustive)

- Signature verification fails (wrong signing key, malformed base64url, or wrong canonical string).
- `X-NBR-Timestamp` outside ±5 minutes.
- Reused `(bot_id, nonce)` within 10 minutes.
- `X-NBR-Body-SHA256` does not match raw body bytes.
- Idempotency collision: same `X-Idempotency-Key` with different body hash.
- `job_id` reuse on `POST /v0/jobs` with different body.
- Payload envelope `recipient_bot_id` does not match job counterparty.
- Inner payload `payload_id`/`job_id`/`sender_bot_id`/`recipient_bot_id` mismatch with outer envelope.
- Inner payload signature fails.
- Charge signature fails or seller_bot_id mismatch.
- Charge attached to non-REQUESTED job.
- Deliver attempted before PAID.
- Reused payload_id with different ciphertext.
- Poll `since_event_id` older than retention (must return 410 with min_event_id_retained).
