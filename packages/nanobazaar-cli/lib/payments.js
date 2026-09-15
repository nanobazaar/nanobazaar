'use strict';

const crypto = require('node:crypto');
const ADDRESS = /^nano_[13][13456789abcdefghijkmnopqrstuwxyz]{59}$/;
const HASH = /^[A-Fa-f0-9]{64}$/;

function timestamp(now = Date.now()) {
  return new Date(now).toISOString().replace(/\.000Z$/, 'Z').replace(/(\.\d*[1-9])0+Z$/, '$1Z');
}

function raw(value, label, allowZero = false) {
  if (typeof value !== 'string' || !/^(0|[1-9]\d{0,38})$/.test(value)) throw new Error(`${label} must be an integer raw string`);
  const n = BigInt(value);
  if ((!allowZero && n === 0n) || n > (1n << 128n) - 1n) throw new Error(`${label} is outside the Nano amount range`);
  return n;
}

function fingerprint(publicKey) {
  const bytes = Buffer.from(publicKey, 'base64url');
  if (bytes.length !== 32 || bytes.toString('base64url') !== publicKey) throw new Error('Invalid seller signing key');
  const alphabet = 'abcdefghijklmnopqrstuvwxyz234567';
  let bits = 0, acc = 0, encoded = 'b';
  for (const byte of crypto.createHash('sha256').update(bytes).digest()) {
    acc = (acc << 8) | byte;
    bits += 8;
    while (bits >= 5) { encoded += alphabet[(acc >>> (bits - 5)) & 31]; bits -= 5; }
  }
  if (bits) encoded += alphabet[(acc << (5 - bits)) & 31];
  return encoded;
}

function future(value, label, now) {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}T.*Z$/.test(value) || !Number.isFinite(Date.parse(value)) || Date.parse(value) <= now) {
    throw new Error(`${label} is invalid or expired`);
  }
}

function deadlineKey(value) {
  const match = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?Z$/.exec(value);
  return match ? `${match[1]}.${(match[2] || '').padEnd(9, '0')}` : `${Date.parse(value)}`.padStart(20, '0');
}

function verifyCharge(job, seller, buyerId, canonical, now = Date.now()) {
  if (!job || job.buyer_bot_id !== buyerId) throw new Error('Job buyer does not match this bot');
  if (job.status !== 'CHARGE_CREATED') throw new Error('Job is not payable (expected CHARGE_CREATED)');
  const charge = job.charge;
  if (!charge || !ADDRESS.test(charge.address)) throw new Error('Missing or invalid Nano charge address');
  for (const value of [job.job_id, job.offer_id, job.seller_bot_id, job.buyer_bot_id, charge.charge_id]) {
    if (typeof value !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/.test(value)) throw new Error('Invalid charge identity field');
  }
  if (raw(charge.amount_raw, 'charge amount') !== raw(job.price_raw, 'job price')) throw new Error('Charge amount differs from agreed job price');
  future(charge.charge_expires_at, 'Charge', now);
  future(job.job_expires_at, 'Job', now);
  if (!seller || fingerprint(seller.signing_pubkey_ed25519) !== job.seller_bot_id) throw new Error('Seller signing key fingerprint does not match bot_id');
  const signature = Buffer.from(charge.charge_sig_ed25519 || '', 'base64url');
  if (signature.length !== 64 || signature.toString('base64url') !== charge.charge_sig_ed25519) throw new Error('Invalid charge signature encoding');
  const message = canonical({jobId: job.job_id, offerId: job.offer_id, sellerBotId: job.seller_bot_id, buyerBotId: job.buyer_bot_id, chargeId: charge.charge_id, address: charge.address, amountRaw: charge.amount_raw, chargeExpiresAt: charge.charge_expires_at});
  const key = crypto.createPublicKey({format: 'jwk', key: {kty: 'OKP', crv: 'Ed25519', x: seller.signing_pubkey_ed25519}});
  if (!crypto.verify(null, Buffer.from(message), key, signature)) throw new Error('Charge signature verification failed');
  return {...charge, job_id: job.job_id, offer_id: job.offer_id, seller_bot_id: job.seller_bot_id, buyer_bot_id: buyerId, job_expires_at: job.job_expires_at};
}

function reservePayment(journal, intent, policy, now = Date.now()) {
  if (!policy || policy.buyer_bot_id !== intent.buyer_bot_id) throw new Error('Policy must authorize this buyer_bot_id');
  if (!Array.isArray(policy.allowed_sellers) || !policy.allowed_sellers.includes(intent.seller_bot_id)) throw new Error('Seller is not authorized by policy');
  future(policy.expires_at, 'Policy', now);
  const amount = raw(intent.amount_raw, 'amount_raw');
  const max = raw(policy.max_payment_raw, 'max_payment_raw', true);
  const budget = raw(policy.total_budget_raw, 'total_budget_raw', true);
  if (amount > max) throw new Error('Payment exceeds policy per-payment limit');
  return journal.transaction(data => {
    future(policy.expires_at, 'Policy', Date.now());
    future(intent.charge_expires_at, 'Charge', Date.now());
    future(intent.job_expires_at, 'Job', Date.now());
    if (Object.hasOwn(data.payments, intent.job_id)) return {created: false, payment: structuredClone(data.payments[intent.job_id])};
    if (data.payer_address && data.payer_address !== intent.payer_address) throw new Error('Wallet address changed; use the original payer for this journal');
    let reserved = 0n;
    for (const payment of Object.values(data.payments)) {
      reserved += raw(payment.amount_raw, 'reserved amount');
      if (payment.address === intent.address) throw new Error('Charge address was already used for another job. Request a fresh charge address.');
    }
    if (reserved + amount > budget) throw new Error('Payment exceeds remaining authorized total budget');
    data.payer_address = intent.payer_address;
    // Persist before emitting the only actionable handoff. A crash after the
    // handoff may conceal a completed external send, so this reservation is
    // deliberately never treated as permission to prepare another send.
    const sendDeadline = [policy.expires_at, intent.charge_expires_at, intent.job_expires_at]
      .reduce((earliest, value) => deadlineKey(value) < deadlineKey(earliest) ? value : earliest);
    const payment = {
      ...intent,
      reservation_id: crypto.randomUUID(),
      status: 'reserved_unknown',
      reserved_at: timestamp(now),
      send_deadline: sendDeadline,
      policy: structuredClone(policy),
    };
    data.payments[intent.job_id] = payment;
    return {created: true, payment: structuredClone(payment)};
  });
}

function rpcUrl(value) {
  if (!value) throw new Error('Set NBR_NANO_RPC_URL to a trusted Nano RPC before payment preparation or chain verification');
  const url = new URL(value);
  if (url.protocol !== 'https:' && !(url.protocol === 'http:' && ['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname))) throw new Error('Nano RPC must use HTTPS (HTTP allowed only on localhost)');
  return url.href;
}

function verifyBlock(block, intent) {
  const contents = typeof block.contents === 'string' ? JSON.parse(block.contents) : block.contents;
  if (block.confirmed !== 'true' && block.confirmed !== true) throw new Error('Payment block is not confirmed');
  if (block.subtype !== 'send') throw new Error('Payment block is not a send');
  if (block.block_account !== intent.payer_address || !contents || contents.account !== intent.payer_address) throw new Error('Payment block has the wrong payer');
  if (contents.link_as_account !== intent.address) throw new Error('Payment block has the wrong destination');
  if (raw(block.amount, 'block amount') !== raw(intent.amount_raw, 'approved amount')) throw new Error('Payment block has the wrong amount');
  if (!/^\d+$/.test(block.height || '') || !/^\d+$/.test(intent.payer_block_count || '') || BigInt(block.height) <= BigInt(intent.payer_block_count)) throw new Error('Payment block does not follow the saved payer chain position');
}

async function payerBoundary(nanoRpcUrl, payerAddress, fetchImpl = fetch) {
  if (!ADDRESS.test(payerAddress || '')) throw new Error('Invalid Nano payer address');
  const response = await fetchImpl(rpcUrl(nanoRpcUrl), {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({action: 'account_info', account: payerAddress}), signal: AbortSignal.timeout(15000)});
  if (!response.ok) throw new Error('Cannot read payer chain position before payment');
  const account = await response.json();
  // An unopened account can receive pending funds before its first send.
  if (account.error === 'Account not found') return '0';
  if (account.error || typeof account.block_count !== 'string' || !/^(0|[1-9]\d*)$/.test(account.block_count)) throw new Error('Invalid payer chain position from Nano RPC');
  return account.block_count;
}

async function inspectBlock(nanoRpcUrl, hash, fetchImpl = fetch) {
  if (!HASH.test(hash || '')) throw new Error('Invalid Nano block hash');
  const response = await fetchImpl(rpcUrl(nanoRpcUrl), {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({action: 'block_info', json_block: 'true', hash: hash.toUpperCase()}), signal: AbortSignal.timeout(15000)});
  if (!response.ok) throw new Error(`Nano RPC verification failed (${response.status}); payment remains reserved`);
  const block = await response.json();
  if (block.error) throw new Error('Nano RPC could not verify the block; payment remains reserved');
  return block;
}

async function verifyUnusedAddress(nanoRpcUrl, address, fetchImpl = fetch) {
  if (!ADDRESS.test(address)) throw new Error('Invalid Nano charge address');
  const request = async body => {
    const response = await fetchImpl(rpcUrl(nanoRpcUrl), {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body), signal: AbortSignal.timeout(15000)});
    if (!response.ok) throw new Error('Cannot verify unused charge address');
    return response.json();
  };
  const pending = await request({action: 'receivable', account: address, count: '1', include_active: 'true', include_only_confirmed: 'false'});
  const empty = pending.blocks === '' || pending.blocks && typeof pending.blocks === 'object' && Object.keys(pending.blocks).length === 0;
  if (pending.error || !empty) throw new Error('Charge address has receivable funds or could not be verified as unused');
  // Read account_info last: if a listener received an old pending transfer while
  // receivable was queried, the account now exists and must be rejected too.
  const account = await request({action: 'account_info', account: address});
  if (account.error !== 'Account not found') throw new Error('Charge address is already opened or could not be verified as unused');
  return {verified_unused: true, checked_at: timestamp()};
}

async function reconcilePayment({journal, jobId, blockHash, nanoRpcUrl, notify, fetchImpl = fetch}) {
  const entries = journal.read().payments;
  if (!Object.hasOwn(entries, jobId)) throw new Error('No saved payment attempt for this job');
  let attempt = entries[jobId];
  const hash = blockHash || attempt.block_hash;
  if (!hash) return {...attempt, send_authorized: false, action_required: 'Locate the original send block hash and run job reconcile --block-hash. Do not send again.'};
  if (!HASH.test(hash)) throw new Error('Invalid Nano block hash');
  const normalized = hash.toUpperCase();
  if (attempt.block_hash && normalized !== attempt.block_hash) throw new Error('Block hash differs from the saved payment receipt');
  if (attempt.status !== 'confirmed') {
    const block = await inspectBlock(nanoRpcUrl, normalized, fetchImpl);
    verifyBlock(block, attempt);
    journal.transaction(data => {
      for (const [id, payment] of Object.entries(data.payments)) {
        if (id !== jobId && payment.block_hash === normalized) throw new Error('Payment block already belongs to another job');
      }
      const current = data.payments[jobId];
      if (current.block_hash && current.block_hash !== normalized) throw new Error('Concurrent receipt mismatch');
      current.block_hash = normalized;
      current.status = 'confirmed';
      current.evidence = current.evidence || {verifier: 'nano_rpc', observed_at: timestamp(), payment_block_hash: normalized, amount_raw_received: current.amount_raw};
    });
  }
  attempt = journal.read().payments[jobId];
  if (attempt.notification_status !== 'notified') {
    try {
      await notify(attempt);
      journal.transaction(data => { data.payments[jobId].notification_status = 'notified'; delete data.payments[jobId].notification_error; });
    } catch (err) {
      journal.transaction(data => {
        if (data.payments[jobId].notification_status !== 'notified') {
          data.payments[jobId].notification_status = 'blocked';
          data.payments[jobId].notification_error = err.message;
        }
      });
    }
  }
  return {...journal.read().payments[jobId], send_authorized: false};
}

module.exports = {verifyCharge, reservePayment, verifyBlock, reconcilePayment, rpcUrl, fingerprint, payerBoundary, inspectBlock, verifyUnusedAddress};
