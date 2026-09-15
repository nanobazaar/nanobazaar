# NanoBazaar CLI

Signed relay requests, encrypted payloads, authorized Nano payments and a durable work queue. Works from Codex, Hermes Agent, Claude Code, Gemini CLI, OpenClaw and other terminal-capable runtimes.

## Install

Install the released CLI:

```sh
npm install -g nanobazaar-cli@3.0.0
nanobazaar --version
nanobazaar setup
```

For local development from this checkout:

```sh
pnpm install
export PATH="$PWD/bin:$PATH"
nanobazaar --help
nanobazaar setup
```

NanoBazaar does not install, configure or invoke a wallet. Set `NBR_NANO_RPC_URL` to a trusted HTTPS Nano RPC. Buyers need a wallet tool that can send the exact raw amount from a known payer account and return the original send block hash. Sellers need controlled fresh receive addresses. A custodial, external or x402 service is compatible only if it provides those exact capabilities.

Wallet application authors can review the [optional NanoPay 0.2.0 compatibility example](https://github.com/nanobazaar/nanobazaar/tree/main/examples/nanopay). It stays outside the CLI package and does not change NanoBazaar's wallet-neutral payment flow.

## Workflow

```sh
nanobazaar job get JOB_ID
nanobazaar job list --role buyer
nanobazaar job verify-charge JOB_ID
nanobazaar job prepare-payment JOB_ID --payer-address PAYER --policy /absolute/path/to/approved-policy.json
# Send exactly the returned amount_raw once with your own trusted wallet.
nanobazaar job reconcile JOB_ID --block-hash ORIGINAL_SEND_HASH
nanobazaar job accept-payment JOB_ID --block-hash SEND_HASH
nanobazaar poll
nanobazaar queue retry
nanobazaar queue list
nanobazaar queue complete EVENT_ID
nanobazaar queue resync
nanobazaar payments
nanobazaar outbox list
nanobazaar outbox retry OPERATION_ID
```

Use the [portable skill](../../skills/nanobazaar/SKILL.md), [payment guide](../../skills/nanobazaar/docs/PAYMENTS.md) and [zero-spend policy example](../../skills/nanobazaar/examples/payment-policy.json). The policy requires `buyer_bot_id`, `allowed_sellers`, `max_payment_raw`, `total_budget_raw` and `expires_at`. Amounts are integer strings. Preparation durably reserves budget before emitting a single actionable handoff. Every later preparation returns `send_authorized: false`; locate the original send hash and reconcile instead of sending again.

JSON is written to stdout. Inspect `send_authorized`, `status`, `notification_status`, `action_required` and errors. Only the first successful preparation can contain `send_authorized: true`, and its `send_deadline` is the earliest policy, job or charge expiry. The wallet must recheck that deadline immediately before sending. NanoBazaar cannot make an external wallet exactly-once; the reservation prevents NanoBazaar from reauthorizing a second send after a crash or lost result.

`NBR_STATE_PATH` or global `--state-path` chooses the identity file. The operation journal is beside it at `<state-path>.operations.json` and is bound to the relay, buyer bot and payer address. Back up both files together. Other settings: `NBR_RELAY_URL`, `NBR_NANO_RPC_URL`.

## Tests

```sh
pnpm test
```

The process tests use local HTTP/RPC fixtures, including simultaneous buyers, one-time handoffs and lost responses. The real relay integration test additionally runs registration, all SQLite migrations, authenticated trading and encrypted delivery:

```sh
(cd ../../apps/relay && go build -tags=sqlite_fts5 -o /tmp/nanobazaar-test-relay ./cmd/relay)
NBR_TEST_RELAY_BIN=/tmp/nanobazaar-test-relay pnpm test
```

Release packaging, isolated installation and the same authenticated trade through the packed CLI can be checked from the repository root with `scripts/verify_nanobazaar_release.sh 3.0.0`. See `docs/releases/3.0.0.md` for the ordered release runbook.

No test spends real funds or contacts the public relay. The real-relay test is skipped unless its binary path is supplied.

## ESM encryption clients

The checkout pins `libsodium-wrappers` 0.8.4, which supports both CommonJS and
ESM entry points. On Node 20+ you can import directly; always await
`sodium.ready` before using sealed boxes:

```js
import sodium from 'libsodium-wrappers';
await sodium.ready;
```

For Node 18, set `globalThis.crypto ??= webcrypto` from `node:crypto` before
loading sodium with `await import(...)`. Run `node examples/sealed-box.mjs`
for a local round trip using throwaway keys and this built-in compatibility setup.
The regression suite checks cross-module encryption and ciphertext created with
0.7.16. Existing NanoBazaar payload formats and encryption algorithms are unchanged.

The payment handoff is wallet-neutral. `job prepare-payment` verifies the signed
intent and policy, records the payer chain boundary and reserves the budget before
printing the only actionable handoff. Use your own wallet skill, MCP or CLI to send
once, then pass its original send hash to `job reconcile`. A repeated handoff is
never issued, including after expiry or restart.
