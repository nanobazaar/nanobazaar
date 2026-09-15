# Optional NanoPay compatibility example

This isolated Node.js 22+ example shows how a wallet application can consume one fresh NanoBazaar payment handoff with NanoPay 0.2.0. It is an optional integration sample, not a NanoBazaar wallet adapter or a second payment path. NanoBazaar does not install, configure or invoke it.

NanoPay 0.2.0 is pinned only in this folder. The dependency is GPL-3.0-only; review its license and security model before copying or distributing an integration. It defaults to BerryPay's public RPC when no URL is supplied; this wrapper rejects missing RPC configuration and accepts only HTTPS or localhost test URLs.

## Install and test

```sh
cd examples/nanopay
pnpm install --frozen-lockfile
pnpm test
```

Tests use deterministic throwaway accounts and a localhost mock Nano RPC. They never contact a public RPC or wallet and never spend funds. The mock supplies a public proof-of-work fixture and exercises the actual published `nanopay@0.2.0` package.

## Wallet-owned integration

Call `sendFreshNanoBazaarHandoff` only with the immediate JSON result of `nanobazaar job prepare-payment` when `send_authorized` is exactly `true`. Supply:

- the complete handoff;
- an explicitly chosen trusted `rpcUrl`;
- the wallet's signer object `{ publicKey, sign(hash) }`;
- wallet-owned persistence implementing `claim`, `recordCandidate` and `recordSubmitted`.

`claim` must atomically and durably return `true` only when it creates the first marker for `reservationId`. It must return `false` after a restart or from a concurrent process when that reservation already exists. `recordCandidate` must durably save `blockHash` before it resolves; `recordSubmitted` saves node acceptance. The test's in-memory set demonstrates call order only and is not safe storage for a real wallet.

```js
import { sendFreshNanoBazaarHandoff } from './index.js'

const result = await sendFreshNanoBazaarHandoff({
  handoff: freshPreparePaymentResult,
  rpcUrl: operatorSelectedTrustedRpcUrl,
  signer: callerOwnedWalletSigner,
  persistence: callerOwnedDurableTransactionStore,
})

// status is only "submitted". Independently verify through NanoBazaar.
console.log(result.blockHash)
```

The wrapper snapshots the payer, recipient, exact raw amount, reservation and deadline before awaiting wallet code. It claims the reservation before any RPC, validates and records the signed candidate hash before work or publication, submits once, and never retries. It rechecks the deadline around signing and work and aborts in-flight work/publication when the deadline passes. An abort cannot recall a block that the node already received, so the saved hash remains the recovery key.

On any error carrying `blockHash`, the wallet marker remains consumed. Look up that exact hash in the wallet or trusted RPC, then run:

```sh
nanobazaar job reconcile JOB_ID --block-hash ORIGINAL_SEND_HASH
```

Never call the wrapper again with a saved handoff. `submitted` means the node accepted the block; it does not mean confirmed. NanoBazaar reconciliation independently checks confirmation, payer, payer chain position, destination and exact raw amount.
