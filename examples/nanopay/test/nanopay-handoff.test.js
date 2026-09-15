import assert from 'node:assert/strict'
import http from 'node:http'
import { after, before, test } from 'node:test'
import { accountFromPrivateKey, deriveAddress, hashBlock, signHash } from 'nanopay'
import payments from '../../../packages/nanobazaar-cli/lib/payments.js'
import { NanoPayHandoffError, sendFreshNanoBazaarHandoff } from '../index.js'

const payer = accountFromPrivateKey('0'.repeat(64))
const recipient = accountFromPrivateKey('1'.repeat(64))
const frontier = '55DB97D030018BB0A7D3652A28EE16ADCF3B5D3064C92707DFBD2F2025624A8D'
const validWork = '509f99dedf9e83ad'
const initialBalanceRaw = '100000000000000000000'
const amountRaw = '9007199254740993'
const resultingBalanceRaw = '99990992800745259007'
let server
let rpcUrl
let processMode
let calls
let processedBlock
let onWorkGenerate

before(async () => {
  server = http.createServer(async (request, response) => {
    let raw = ''
    for await (const chunk of request) raw += chunk
    const body = JSON.parse(raw)
    calls.push(body)
    response.setHeader('Content-Type', 'application/json')
    if (body.action === 'account_info') {
      response.end(JSON.stringify({
        frontier,
        balance: initialBalanceRaw,
        representative: payer.address,
        block_count: '1',
        confirmed_frontier: frontier,
        confirmed_balance: initialBalanceRaw,
        confirmed_height: '1',
      }))
      return
    }
    if (body.action === 'work_generate') {
      onWorkGenerate?.()
      response.end(JSON.stringify({ work: validWork }))
      return
    }
    if (body.action === 'process') {
      processedBlock = body.block
      if (processMode === 'ambiguous') {
        response.statusCode = 503
        response.end('{}')
        return
      }
      response.end(JSON.stringify({ hash: hashBlock(body.block) }))
      return
    }
    if (body.action === 'block_info') {
      assert.ok(processedBlock, 'block_info requires a block previously submitted to process')
      assert.equal(body.hash, hashBlock(processedBlock))
      response.end(JSON.stringify({
        confirmed: 'true',
        subtype: 'send',
        block_account: processedBlock.account,
        height: '2',
        amount: String(BigInt(initialBalanceRaw) - BigInt(processedBlock.balance)),
        contents: {
          ...processedBlock,
          link_as_account: deriveAddress(processedBlock.link),
        },
      }))
      return
    }
    assert.fail(`Unexpected Nano RPC action: ${body.action}`)
  })
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  rpcUrl = `http://127.0.0.1:${server.address().port}`
})

after(async () => {
  server.closeAllConnections()
  await new Promise(resolve => server.close(resolve))
})

function handoff(overrides = {}) {
  return {
    send_authorized: true,
    reservation_id: 'reservation-test-1',
    payer_address: payer.address,
    recipient_address: recipient.address,
    amount_raw: amountRaw,
    send_deadline: new Date(Date.now() + 60_000).toISOString(),
    ...overrides,
  }
}

function walletState() {
  const claimed = new Set()
  const records = []
  return {
    records,
    persistence: {
      async claim(entry) {
        if (claimed.has(entry.reservationId)) return false
        claimed.add(entry.reservationId)
        records.push({ phase: 'claimed', ...entry })
        return true
      },
      async recordCandidate(entry) { records.push({ phase: 'candidate', ...entry }) },
      async recordSubmitted(entry) { records.push({ phase: 'submitted', ...entry }) },
    },
  }
}

const signer = {
  publicKey: payer.publicKey,
  sign(hash) { return signHash(hash, '0'.repeat(64)) },
}

test('published NanoPay sends exact handoff fields once and its hash passes NanoBazaar verification', async () => {
  calls = []
  processMode = 'success'
  processedBlock = undefined
  onWorkGenerate = undefined
  const state = walletState()
  const source = handoff()
  const pending = sendFreshNanoBazaarHandoff({ handoff: source, rpcUrl, signer, persistence: state.persistence })
  source.amount_raw = '9'
  source.recipient_address = payer.address
  const result = await pending

  assert.equal(result.status, 'submitted')
  assert.deepEqual(state.records.map(record => record.phase), ['claimed', 'candidate', 'submitted'])
  const process = calls.find(call => call.action === 'process')
  assert.equal(process.block.account, payer.address)
  assert.equal(process.block.link, recipient.publicKey)
  assert.equal(process.block.balance, resultingBalanceRaw)
  assert.equal(hashBlock(process.block), result.blockHash)
  assert.equal(calls.filter(call => call.action === 'process').length, 1)

  const block = await payments.inspectBlock(rpcUrl, result.blockHash)
  assert.doesNotThrow(() => payments.verifyBlock(block, {
    payer_address: payer.address,
    address: recipient.address,
    amount_raw: amountRaw,
    payer_block_count: '1',
  }))

  await assert.rejects(
    sendFreshNanoBazaarHandoff({ handoff: handoff(), rpcUrl, signer, persistence: state.persistence }),
    /already contains this reservation/,
  )
  assert.equal(calls.filter(call => call.action === 'process').length, 1)
})

test('ambiguous process failure preserves the original hash and repeat claim cannot process again', async () => {
  calls = []
  processMode = 'ambiguous'
  processedBlock = undefined
  onWorkGenerate = undefined
  const state = walletState()
  const error = await sendFreshNanoBazaarHandoff({ handoff: handoff(), rpcUrl, signer, persistence: state.persistence }).catch(value => value)
  assert.ok(error instanceof NanoPayHandoffError)
  assert.match(error.blockHash, /^[A-F0-9]{64}$/)
  assert.equal(error.status, 'unknown')
  assert.equal(state.records.find(record => record.phase === 'candidate').blockHash, error.blockHash)
  assert.equal(calls.filter(call => call.action === 'process').length, 1)

  await assert.rejects(
    sendFreshNanoBazaarHandoff({ handoff: handoff(), rpcUrl, signer, persistence: state.persistence }),
    /already contains this reservation/,
  )
  assert.equal(calls.filter(call => call.action === 'process').length, 1)
})

test('a candidate hash remains recoverable when durable candidate recording reports failure', async () => {
  calls = []
  processMode = 'success'
  processedBlock = undefined
  onWorkGenerate = undefined
  const state = walletState()
  state.persistence.recordCandidate = async entry => {
    state.records.push({ phase: 'candidate', ...entry })
    throw new Error('durable store acknowledgement lost')
  }
  const error = await sendFreshNanoBazaarHandoff({ handoff: handoff(), rpcUrl, signer, persistence: state.persistence }).catch(value => value)
  assert.ok(error instanceof NanoPayHandoffError)
  assert.equal(error.blockHash, state.records.find(record => record.phase === 'candidate').blockHash)
  assert.equal(calls.filter(call => call.action === 'process').length, 0)
})

test('fails closed on stale or mismatched handoffs and never falls back to NanoPay default RPC', async () => {
  calls = []
  processMode = 'success'
  processedBlock = undefined
  onWorkGenerate = undefined
  for (const [input, error] of [
    [{ ...handoff(), send_authorized: false }, /send_authorized:true/],
    [handoff({ payer_address: recipient.address }), /public key does not match/],
    [handoff({ recipient_address: 'nano_bad' }), /recipient_address/],
    [handoff({ amount_raw: '07' }), /canonical Nano raw/],
    [handoff({ send_deadline: new Date(Date.now() - 1000).toISOString() }), /deadline has passed/],
  ]) {
    await assert.rejects(sendFreshNanoBazaarHandoff({ handoff: input, rpcUrl, signer, persistence: walletState().persistence }), error)
  }
  await assert.rejects(
    sendFreshNanoBazaarHandoff({ handoff: handoff(), signer, persistence: walletState().persistence }),
    /rpcUrl is required/,
  )
  assert.equal(calls.length, 0)
})

test('deadline is rechecked after work and blocks process while preserving the candidate hash', async () => {
  calls = []
  processMode = 'success'
  processedBlock = undefined
  const state = walletState()
  const deadline = Date.now() + 60_000
  let currentNow = deadline - 1
  onWorkGenerate = () => { currentNow = deadline + 1 }
  const error = await sendFreshNanoBazaarHandoff({
    handoff: handoff({ send_deadline: new Date(deadline).toISOString() }),
    rpcUrl,
    signer,
    persistence: state.persistence,
    now: () => currentNow,
  }).catch(value => value)
  assert.ok(error instanceof NanoPayHandoffError)
  assert.equal(state.records.some(record => record.phase === 'candidate'), true)
  assert.equal(calls.filter(call => call.action === 'process').length, 0)
})
