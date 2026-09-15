'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const {verifyCharge, reservePayment, verifyBlock, verifyUnusedAddress, reconcilePayment} = require('../lib/payments');
const {canonicalChargeString} = require('../bin/nanobazaar');
const {Journal} = require('../lib/journal');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

function fixture(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'nbr-pay-'));
  t.after(() => fs.rmSync(dir, {recursive: true, force: true}));
  const pair = crypto.generateKeyPairSync('ed25519');
  const pub = pair.publicKey.export({format: 'jwk'}).x;
  const alphabet = 'abcdefghijklmnopqrstuvwxyz234567';
  const bits = [...crypto.createHash('sha256').update(Buffer.from(pub, 'base64url')).digest()].map(x => x.toString(2).padStart(8, '0')).join('');
  const seller = 'b' + (bits + '0000').match(/.{5}/g).map(x => alphabet[parseInt(x, 2)]).join('');
  const charge = {charge_id: 'charge1', address: 'nano_1' + '1'.repeat(59), amount_raw: '100000000000000000000000001', charge_expires_at: '2099-01-01T00:00:00.123456789Z'};
  const job = {job_id: 'job1', offer_id: 'offer1', buyer_bot_id: 'buyer', seller_bot_id: seller, status: 'CHARGE_CREATED', price_raw: charge.amount_raw, job_expires_at: '2099-01-02T00:00:00Z', charge};
  charge.charge_sig_ed25519 = crypto.sign(null, Buffer.from(canonicalChargeString({jobId: job.job_id, offerId: job.offer_id, buyerBotId: job.buyer_bot_id, sellerBotId: seller, chargeId: charge.charge_id, address: charge.address, amountRaw: charge.amount_raw, chargeExpiresAt: charge.charge_expires_at})), pair.privateKey).toString('base64url');
  const policy = {buyer_bot_id: 'buyer', allowed_sellers: [seller], max_payment_raw: charge.amount_raw, total_budget_raw: charge.amount_raw, expires_at: '2099-01-01T00:00:00Z'};
  return {job, bot: {signing_pubkey_ed25519: pub}, policy, journal: new Journal(path.join(dir, 'ops.json'), 'relay|buyer')};
}

test('verifies real signatures including nanosecond timestamps and rejects altered charges', t => {
  const {job, bot} = fixture(t);
  assert.equal(verifyCharge(job, bot, 'buyer', canonicalChargeString).amount_raw, job.price_raw);
  for (const change of [
    {buyer_bot_id: 'other'}, {status: 'PAID'}, {price_raw: '1'},
    {charge: {...job.charge, address: 'nano_3' + '1'.repeat(59)}},
    {charge: {...job.charge, charge_expires_at: '2020-01-01T00:00:00Z'}},
  ]) assert.throws(() => verifyCharge({...job, ...change}, bot, 'buyer', canonicalChargeString));
  const other = crypto.generateKeyPairSync('ed25519').publicKey.export({format: 'jwk'}).x;
  assert.throws(() => verifyCharge(job, {signing_pubkey_ed25519: other}, 'buyer', canonicalChargeString), /fingerprint/);
});

test('reserves exact integer budget once and repeated preparation cannot reauthorize a send', t => {
  const {job, policy, journal} = fixture(t);
  const intent = {...job.charge, job_id: job.job_id, seller_bot_id: job.seller_bot_id, buyer_bot_id: 'buyer', payer_address: 'payer', payer_block_count: '7', relay_url: 'https://relay.example', job_expires_at: job.job_expires_at};
  const first = reservePayment(journal, intent, policy);
  assert.equal(first.created, true);
  assert.equal(first.payment.status, 'reserved_unknown');
  assert.equal(first.payment.send_deadline, policy.expires_at);
  const repeated = reservePayment(journal, intent, policy);
  assert.equal(repeated.created, false);
  assert.equal(repeated.payment.reservation_id, first.payment.reservation_id);
  assert.throws(() => reservePayment(journal, {...intent, job_id: 'job2', address: 'different'}, policy), /budget/);
  assert.equal(journal.read().payments.job1.status, 'reserved_unknown');
});

test('handoff deadline is the exact earliest policy, job or charge expiry', t => {
  const variants = [
    ['policy', '2098-12-31T23:59:59.999999999Z'],
    ['job', '2098-12-31T23:59:59.999999998Z'],
    ['charge', '2098-12-31T23:59:59.999999997Z'],
  ];
  for (const [field, expected] of variants) {
    const {job, policy, journal} = fixture(t);
    const intent = {...job.charge, job_id: job.job_id, seller_bot_id: job.seller_bot_id, buyer_bot_id: 'buyer', payer_address: 'payer', payer_block_count: '7', relay_url: 'https://relay.example', job_expires_at: '2099-01-02T00:00:00Z'};
    policy.expires_at = '2099-01-03T00:00:00Z';
    intent.charge_expires_at = '2099-01-01T00:00:00Z';
    if (field === 'policy') policy.expires_at = expected;
    if (field === 'job') intent.job_expires_at = expected;
    if (field === 'charge') intent.charge_expires_at = expected;
    assert.equal(reservePayment(journal, intent, policy).payment.send_deadline, expected);
  }
});

test('payer is fixed for the journal and legacy payment states remain non-sendable', t => {
  const {job, policy, journal} = fixture(t);
  policy.total_budget_raw = (BigInt(policy.total_budget_raw) * 4n).toString();
  const base = {...job.charge, seller_bot_id: job.seller_bot_id, buyer_bot_id: 'buyer', payer_address: 'nano_1' + '1'.repeat(59), payer_block_count: '7', relay_url: 'https://relay.example', job_expires_at: job.job_expires_at};
  reservePayment(journal, {...base, job_id: 'job1'}, policy);
  assert.throws(() => reservePayment(journal, {...base, job_id: 'job2', charge_id: 'charge2', address: 'nano_3' + '3'.repeat(59), payer_address: 'nano_3' + '1'.repeat(59)}, policy), /Wallet address changed/);
  for (const status of ['unknown', 'submitted', 'confirmed']) {
    journal.transaction(data => { data.payments.job1.status = status; });
    const repeated = reservePayment(journal, {...base, job_id: 'job1'}, policy);
    assert.equal(repeated.created, false);
    assert.equal(repeated.payment.status, status);
    assert.equal(journal.read().payments.job1.status, status);
  }
});

test('policy must authorize exact buyer, seller, amount and current time', t => {
  const {job, policy, journal} = fixture(t);
  const intent = {...job.charge, job_id: 'job1', seller_bot_id: job.seller_bot_id, buyer_bot_id: 'buyer', payer_address: 'payer', payer_block_count: '7', relay_url: 'https://relay.example', job_expires_at: job.job_expires_at};
  for (const change of [{buyer_bot_id: 'other'}, {allowed_sellers: []}, {max_payment_raw: '1'}, {total_budget_raw: 100}, {expires_at: '2020-01-01T00:00:00Z'}]) {
    assert.throws(() => reservePayment(journal, intent, {...policy, ...change}));
  }
  assert.deepEqual(journal.read().payments, {});
});

test('authorization expiring while waiting for the journal lock cannot reserve funds', t => {
  const {job, policy, journal} = fixture(t);
  const originalNow = Date.now;
  const before = originalNow();
  let current = before;
  Date.now = () => current;
  try {
    const delayed = {transaction(fn) { current = before + 2000; return journal.transaction(fn); }};
    const intent = {...job.charge, job_id: 'job1', seller_bot_id: job.seller_bot_id, buyer_bot_id: 'buyer', payer_address: 'payer', payer_block_count: '7', relay_url: 'https://relay.example', job_expires_at: job.job_expires_at};
    assert.throws(() => reservePayment(delayed, intent, {...policy, expires_at: new Date(before + 1000).toISOString()}), /Policy.*expired/);
    assert.deepEqual(journal.read().payments, {});
  } finally { Date.now = originalNow; }
});

test('job expiry is rechecked while waiting for the journal lock', t => {
  const {job, policy, journal} = fixture(t);
  const originalNow = Date.now;
  const before = originalNow();
  let current = before;
  Date.now = () => current;
  try {
    const delayed = {transaction(fn) { current = before + 2000; return journal.transaction(fn); }};
    const intent = {...job.charge, job_id: 'job1', seller_bot_id: job.seller_bot_id, buyer_bot_id: 'buyer', payer_address: 'nano_1' + '1'.repeat(59), payer_block_count: '7', relay_url: 'https://relay.example', job_expires_at: new Date(before + 1000).toISOString()};
    assert.throws(() => reservePayment(delayed, intent, policy), /Job.*expired/);
    assert.deepEqual(journal.read().payments, {});
  } finally { Date.now = originalNow; }
});

test('receipt requires a confirmed send from the payer for the exact destination and amount', () => {
  const intent = {payer_address: 'payer', address: 'payee', amount_raw: '100', payer_block_count: '7'};
  const block = {confirmed: 'true', subtype: 'send', block_account: 'payer', amount: '100', height: '8', contents: {account: 'payer', link_as_account: 'payee'}};
  assert.doesNotThrow(() => verifyBlock(block, intent));
  for (const change of [{height: '7'}, {height: '1'}, {height: undefined}, {confirmed: 'false'}, {subtype: 'receive'}, {amount: '101'}, {block_account: 'other'}, {contents: {account: 'payer', link_as_account: 'other'}}]) assert.throws(() => verifyBlock({...block, ...change}, intent));
});

test('seller charge freshness rejects opened accounts and old unreceived payments', async () => {
  const address = 'nano_1' + '1'.repeat(59);
  const rpc = (pending, account) => async (url, options) => ({ok: true, json: async () => JSON.parse(options.body).action === 'receivable' ? pending : account});
  assert.equal((await verifyUnusedAddress('https://rpc.example', address, rpc({blocks: ''}, {error: 'Account not found'}))).verified_unused, true);
  for (const [pending, account] of [[{blocks: {['A'.repeat(64)]: '100'}}, {error: 'Account not found'}], [{blocks: ''}, {block_count: '1'}], [{error: 'unavailable'}, {error: 'Account not found'}], [{blocks: ''}, {error: 'unavailable'}]]) {
    await assert.rejects(verifyUnusedAddress('https://rpc.example', address, rpc(pending, account)), /unused|receivable/);
  }
});

test('a stale notification failure cannot downgrade a concurrent success', async t => {
  const {journal} = fixture(t);
  journal.transaction(data => { data.payments.job1 = {status: 'confirmed', block_hash: 'A'.repeat(64)}; });
  let rejectLate;
  const first = reconcilePayment({journal, jobId: 'job1', notify: () => new Promise((resolve, reject) => { rejectLate = reject; })});
  const second = await reconcilePayment({journal, jobId: 'job1', notify: async () => {}});
  assert.equal(second.notification_status, 'notified');
  rejectLate(new Error('Late failed response'));
  const result = await first;
  assert.equal(result.notification_status, 'notified');
  assert.equal(result.notification_error, undefined);
});
