import {
  SEND_WORK_THRESHOLD,
  createClient,
  deriveAddress,
  normalizeAddress,
  verifyHash,
} from 'nanopay'

const RAW = /^(0|[1-9]\d{0,38})$/
const MAX_RAW = (1n << 128n) - 1n
const MAX_TIMER_MS = 2_147_483_647

export class NanoPayHandoffError extends Error {
  constructor(message, { blockHash, cause, status = 'unknown' }) {
    super(message, { cause })
    this.name = 'NanoPayHandoffError'
    this.blockHash = blockHash
    this.status = status
  }
}

function exactRaw(value) {
  if (typeof value !== 'string' || !RAW.test(value) || BigInt(value) === 0n || BigInt(value) > MAX_RAW) {
    throw new Error('handoff.amount_raw must be a positive canonical Nano raw string')
  }
  return value
}

function explicitRpcUrl(value) {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error('rpcUrl is required; this example never uses NanoPay default RPC')
  }
  const url = new URL(value)
  const local = url.protocol === 'http:' && ['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname)
  if (url.protocol !== 'https:' && !local) throw new Error('rpcUrl must use HTTPS (localhost HTTP is allowed for tests)')
  return url.href
}

function snapshotInputs({ handoff, signer, persistence }) {
  if (!handoff || handoff.send_authorized !== true) throw new Error('A fresh send_authorized:true handoff is required')
  const snapshot = Object.freeze({
    reservationId: String(handoff.reservation_id || ''),
    payerAddress: String(handoff.payer_address || ''),
    recipientAddress: String(handoff.recipient_address || ''),
    amountRaw: exactRaw(handoff.amount_raw),
    sendDeadline: String(handoff.send_deadline || ''),
  })
  if (!snapshot.reservationId) throw new Error('handoff.reservation_id is required')
  let normalizedPayer
  let normalizedRecipient
  try { normalizedPayer = normalizeAddress(snapshot.payerAddress) } catch {}
  try { normalizedRecipient = normalizeAddress(snapshot.recipientAddress) } catch {}
  if (!snapshot.payerAddress.startsWith('nano_') || normalizedPayer !== snapshot.payerAddress) {
    throw new Error('handoff.payer_address must be a canonical Nano address')
  }
  if (!snapshot.recipientAddress.startsWith('nano_') || normalizedRecipient !== snapshot.recipientAddress) {
    throw new Error('handoff.recipient_address must be a canonical Nano address')
  }
  const deadlineMs = Date.parse(snapshot.sendDeadline)
  if (!Number.isFinite(deadlineMs)) throw new Error('handoff.send_deadline is invalid')

  const publicKey = String(signer?.publicKey || '').toUpperCase()
  if (typeof signer?.sign !== 'function') throw new Error('signer must provide sign(hash)')
  if (deriveAddress(publicKey) !== snapshot.payerAddress) throw new Error('signer public key does not match handoff.payer_address')

  for (const method of ['claim', 'recordCandidate', 'recordSubmitted']) {
    if (typeof persistence?.[method] !== 'function') throw new Error(`persistence.${method} is required`)
  }
  return {
    handoff: snapshot,
    deadlineMs,
    signer: Object.freeze({ publicKey, sign: signer.sign.bind(signer) }),
    persistence: Object.freeze({
      claim: persistence.claim.bind(persistence),
      recordCandidate: persistence.recordCandidate.bind(persistence),
      recordSubmitted: persistence.recordSubmitted.bind(persistence),
    }),
  }
}

function deadlineGuard(deadlineMs, now) {
  if (now() >= deadlineMs) throw new Error('NanoBazaar handoff deadline has passed; do not send')
}

function deadlineSignal(deadlineMs, callerSignal) {
  const controller = new AbortController()
  const delay = deadlineMs - Date.now()
  const timer = delay <= MAX_TIMER_MS
    ? setTimeout(() => controller.abort(new DOMException('NanoBazaar handoff deadline passed', 'TimeoutError')), Math.max(0, delay))
    : null
  const signal = callerSignal ? AbortSignal.any([callerSignal, controller.signal]) : controller.signal
  return { signal, close: () => { if (timer) clearTimeout(timer) } }
}

/**
 * Submit one fresh NanoBazaar handoff through NanoPay 0.2.0.
 *
 * persistence.claim must atomically and durably return true only for the first
 * use of a reservation ID. The other callbacks must durably flush before they
 * resolve. This function never retries and never reports confirmation.
 */
export async function sendFreshNanoBazaarHandoff({
  handoff,
  rpcUrl,
  signer,
  persistence,
  signal: callerSignal,
  timeoutMs = 15_000,
  now = Date.now,
}) {
  const rpc = explicitRpcUrl(rpcUrl)
  const captured = snapshotInputs({ handoff, signer, persistence })
  const { deadlineMs } = captured
  const details = captured.handoff
  deadlineGuard(deadlineMs, now)

  const claimed = await captured.persistence.claim(details)
  if (claimed !== true) throw new Error('Wallet persistence already contains this reservation; reconcile it and do not send again')
  deadlineGuard(deadlineMs, now)

  let candidateHash
  let client
  const timed = deadlineSignal(deadlineMs, callerSignal)
  const recordingSigner = Object.freeze({
    publicKey: captured.signer.publicKey,
    async sign(hash) {
      deadlineGuard(deadlineMs, now)
      const signature = await captured.signer.sign(hash)
      deadlineGuard(deadlineMs, now)
      if (!verifyHash({ hash, signature, publicKey: captured.signer.publicKey })) throw new Error('Signer returned an invalid Nano signature')
      candidateHash = hash
      await captured.persistence.recordCandidate(Object.freeze({ ...details, blockHash: hash }))
      deadlineGuard(deadlineMs, now)
      return signature
    },
  })

  try {
    client = createClient({
      rpcUrl: rpc,
      timeoutMs,
      work: async ({ root, threshold, signal }) => {
        deadlineGuard(deadlineMs, now)
        if (threshold !== SEND_WORK_THRESHOLD) throw new Error('Unexpected Nano work threshold')
        const work = await client.generateWork(root, { threshold, signal })
        deadlineGuard(deadlineMs, now)
        return work
      },
    })
    const sent = await client.send({
      account: recordingSigner,
      to: details.recipientAddress,
      amountRaw: details.amountRaw,
      signal: timed.signal,
    })
    if (sent.status !== 'submitted' || sent.hash !== candidateHash) throw new Error('NanoPay returned an inconsistent submitted transaction')
    try {
      await captured.persistence.recordSubmitted(Object.freeze({ ...details, blockHash: sent.hash, status: 'submitted' }))
    } catch (cause) {
      throw new NanoPayHandoffError('Node accepted the block but submitted state was not saved; reconcile blockHash and do not send again', {
        blockHash: sent.hash,
        status: 'submitted',
        cause,
      })
    }
    return Object.freeze({ reservationId: details.reservationId, blockHash: sent.hash, status: 'submitted' })
  } catch (cause) {
    if (cause instanceof NanoPayHandoffError) throw cause
    const blockHash = cause?.transaction?.hash || candidateHash
    if (blockHash) {
      throw new NanoPayHandoffError('The wallet reservation remains consumed; reconcile blockHash and do not send again', {
        blockHash,
        status: 'unknown',
        cause,
      })
    }
    throw cause
  } finally {
    timed.close()
  }
}
