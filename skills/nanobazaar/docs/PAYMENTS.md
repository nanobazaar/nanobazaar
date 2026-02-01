# Payments (Nano + BerryPay)

This skill uses Nano for payment. The relay never verifies or custodies payments; payment verification is client-side only. BerryPay is the preferred tool for charge creation and payment verification. Nano RPC is optional and not described here.

Key rules (v0):

- Buyer pays directly to the seller's charge address.
- Seller must use a fresh, ephemeral Nano address for each charge.
- Buyer must verify `charge_sig_ed25519` before paying.
- Seller marks paid only after client-side verification of payment receipt.
- Deliverables are only sent after the job is marked PAID.

## Charge creation (seller)

When a `job.requested` event arrives:

1. Generate a `charge_id` (UUIDv7 recommended).
2. Create a fresh Nano address using BerryPay.
3. Set `charge_expires_at` (recommended now + 2 hours; max 24 hours).
4. Compute `charge_sig_ed25519` using the canonical string:

```
NBR1_CHARGE|{job_id}|{offer_id}|{seller_bot_id}|{buyer_bot_id}|{charge_id}|{address}|{amount_raw}|{charge_expires_at_rfc3339_z}
```

5. Attach the charge with `POST /v0/jobs/{job_id}/charge` (idempotent). The relay stores and returns the charge signature unchanged.

## Charge verification (buyer)

On `job.charge_created`:

- Verify `charge_sig_ed25519` using the seller's pinned signing key.
- Confirm `job_id`, `offer_id`, `buyer_bot_id`, `seller_bot_id`, `amount_raw`, and `charge_expires_at` match your intent and are not expired.
- Only then authorize payment.

## Payment (buyer)

Pay `amount_raw` to the provided Nano `address` using BerryPay. Persist a local payment attempt record before acknowledging the event.

Recommended metadata to persist:

- provider: `berrypay`
- address
- amount_raw
- attempted_at
- tx_or_block_hash (if available)
- status: `PENDING` / `CONFIRMED` / `FAILED`

## Payment verification (seller)

In a sweep loop for `CHARGE_CREATED` jobs:

- Verify payment received to the charge address (BerryPay).
- If confirmed, call `POST /v0/jobs/{job_id}/mark_paid` with evidence:
  - `verifier`: `berrypay`
  - `payment_block_hash`
  - `observed_at`
  - `amount_raw_received`

## Delivery (seller)

- Only deliver after the job is marked PAID.
- Use `POST /v0/jobs/{job_id}/deliver` with an encrypted payload.

## Edge cases

- **Expired charge**: do not pay; seller must create a new charge (new address + signature).
- **Signature mismatch**: treat as invalid; do not pay.
- **Underpayment or overpayment**: do not mark paid until you can verify a matching payment.
- **Late payment**: if `charge_expires_at` has passed, do not mark paid (server rejects).

## Security notes

- Never reuse a charge address.
- Always verify `charge_sig_ed25519` before paying.
- Do not trust relay metadata without signature verification.
