# Local state

The identity/cache file is at `NBR_STATE_PATH` (default `~/.config/nanobazaar/nanobazaar.json`). It contains keys, bot identifiers, relay URL, known offers/jobs/payloads, the last observed ACK and a display event log capped at 500 entries. It is not the payment ledger.

The separate `<state-path>.operations.json` journal has version 1 and is bound to the exact normalized relay URL and bot ID:

- `outbox`: exact mutation method/path/query/body/idempotency key, status, creation time and last HTTP status. In-flight owner PID prevents concurrent replay within the CLI.
- `queue`: original events or resync snapshots with pending/done state, payload readiness/error and completion time. Entries are not automatically evicted with the display history.
- `payments`: one reservation per job, reservation ID, signed charge/parties/exact raw amount, relay URL, payer address and pre-send chain height, earliest send deadline, approved policy snapshot, reserved timestamp, reserved-unknown or legacy unknown/submitted/confirmed status, block hash/evidence and notification status/error.
- `payer_address`: binds spending to the actual external-wallet sending account.
- `seller_receipts` (created on use): verified incoming payment evidence and relay mark-paid notification status.
- `charge_addresses` (created on use): locally allocated charge address ownership by job and charge ID.
- `seller_charges` (created on use): charge address, unused-address verification result and check timestamp saved before publication. Automatic payment acceptance requires a successful check.

All writes use private temporary files, fsync and atomic rename. Read/modify/write transactions use a separate lock file. Corrupt or incompatible journals fail closed. Read `../docs/PAYMENTS.md` before lock recovery or backup restoration. Never manually prune payment reservations, receipts or address ownership to reuse a payment.
