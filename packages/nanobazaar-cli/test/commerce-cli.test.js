'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const http = require('node:http');
const crypto = require('node:crypto');
const {spawn} = require('node:child_process');
const {canonicalChargeString} = require('../bin/nanobazaar');
const {fingerprint} = require('../lib/payments');

const cli = path.resolve(__dirname, '../bin/nanobazaar');
const payer = 'nano_1' + '1'.repeat(59);
const address = 'nano_3' + '1'.repeat(59);
const hash = 'A'.repeat(64);

function run(args, env) {
  return new Promise(resolve => {
    const child = spawn(process.execPath, [cli, ...args], {env: {...process.env, ...env, NBR_IDEMPOTENCY_KEY: '', NBR_SIGNING_PRIVATE_KEY_B64URL: '', NBR_SIGNING_PUBLIC_KEY_B64URL: '', NBR_ENCRYPTION_PRIVATE_KEY_B64URL: '', NBR_ENCRYPTION_PUBLIC_KEY_B64URL: ''}});
    let stdout = '', stderr = '';
    child.stdout.on('data', b => { stdout += b; });
    child.stderr.on('data', b => { stderr += b; });
    child.on('close', code => resolve({code, stdout, stderr, json: () => JSON.parse(stdout)}));
  });
}

async function fixture(t, options = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'nbr-commerce-'));
  const signing = crypto.generateKeyPairSync('ed25519').privateKey.export({format: 'jwk'});
  const encryption = crypto.generateKeyPairSync('x25519').privateKey.export({format: 'jwk'});
  const sellerPair = crypto.generateKeyPairSync('ed25519');
  const sellerKey = sellerPair.publicKey.export({format: 'jwk'}).x;
  const buyer = fingerprint(signing.x), seller = fingerprint(sellerKey);
  const statePath = path.join(dir, 'identity.json');
  fs.writeFileSync(statePath, JSON.stringify({keys: {signing_private_key_b64url: signing.d, signing_public_key_b64url: signing.x, encryption_private_key_b64url: encryption.d, encryption_public_key_b64url: encryption.x}}));
  const jobs = {};
  for (const id of ['job1', 'job2']) {
    const charge = {charge_id: 'charge-' + id, address: id === 'job1' ? address : 'nano_1' + '3'.repeat(59), amount_raw: '100000000000000000000000001', charge_expires_at: '2099-01-01T00:00:00.123456789Z'};
    jobs[id] = {job_id: id, offer_id: 'offer1', buyer_bot_id: buyer, seller_bot_id: seller, price_raw: charge.amount_raw, status: 'CHARGE_CREATED', job_expires_at: '2099-01-02T00:00:00Z', charge};
    charge.charge_sig_ed25519 = crypto.sign(null, Buffer.from(canonicalChargeString({jobId: id, offerId: 'offer1', sellerBotId: seller, buyerBotId: buyer, chargeId: charge.charge_id, address: charge.address, amountRaw: charge.amount_raw, chargeExpiresAt: charge.charge_expires_at})), sellerPair.privateKey).toString('base64url');
  }
  const receipts = [], acks = [];
  let notified = false;
  const server = http.createServer(async (req, res) => {
    let body = ''; for await (const chunk of req) body += chunk;
    const send = (status, value) => { res.writeHead(status, {'Content-Type': 'application/json'}); res.end(JSON.stringify(value)); };
    if (req.url === '/rpc') return send(200, JSON.parse(body).action === 'account_info' ? {block_count: '7'} : {confirmed: 'true', subtype: 'send', block_account: payer, height: '8', amount: jobs.job1.price_raw, contents: {account: payer, link_as_account: jobs.job1.charge.address}});
    const signed = `${req.method}\n${req.url}\n${req.headers['x-nbr-timestamp']}\n${req.headers['x-nbr-nonce']}\n${crypto.createHash('sha256').update(body).digest('hex')}`;
    const valid = crypto.verify(null, Buffer.from(signed), crypto.createPublicKey({format: 'jwk', key: {kty: 'OKP', crv: 'Ed25519', x: signing.x}}), Buffer.from(req.headers['x-nbr-signature'] || '', 'base64url'));
    if (!valid) return send(401, {error: 'signature'});
    if (req.url.startsWith('/v0/bots/')) return send(200, {signing_pubkey_ed25519: sellerKey});
    if (/^\/v0\/jobs\/job[12]$/.test(req.url)) return send(200, jobs[req.url.split('/').pop()]);
    if (req.url.endsWith('/payment_sent')) {
      receipts.push({body, key: req.headers['x-idempotency-key'], nonce: req.headers['x-nbr-nonce']});
      if (options.dropNotification && !notified) { notified = true; req.socket.destroy(); return; }
      return send(options.rejectNotification ? 409 : 200, {ok: true});
    }
    if (req.url.startsWith('/v0/poll?') || req.url === '/v0/poll') return send(200, {events: options.events || []});
    if (req.url === '/v0/poll/ack') { acks.push(JSON.parse(body)); return send(200, {last_acked_event_id: JSON.parse(body).up_to_event_id}); }
    if (req.url.startsWith('/v0/payloads/')) return send(503, {error: 'temporarily unavailable'});
    send(404, {error: 'unknown route'});
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const base = `http://127.0.0.1:${server.address().port}`;
  const policyPath = path.join(dir, 'policy.json');
  fs.writeFileSync(policyPath, JSON.stringify({buyer_bot_id: buyer, allowed_sellers: [seller], max_payment_raw: jobs.job1.price_raw, total_budget_raw: jobs.job1.price_raw, expires_at: '2099-01-01T00:00:00Z'}));
  const env = {NBR_STATE_PATH: statePath, NBR_RELAY_URL: base, NBR_NANO_RPC_URL: base + '/rpc'};
  t.after(async () => { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); fs.rmSync(dir, {recursive: true, force: true}); });
  return {env, policyPath, receipts, acks, jobs, statePath};
}

test('real CLI prepares one wallet handoff, verifies an external send and retries identical notification bytes', async t => {
  const f = await fixture(t, {dropNotification: true});
  const first = await run(['job', 'prepare-payment', 'job1', '--payer-address', payer, '--policy', f.policyPath], f.env);
  assert.equal(first.code, 0, first.stderr);
  assert.equal(first.json().send_authorized, true);
  assert.equal(first.json().amount_raw, f.jobs.job1.price_raw);
  assert.equal(first.json().recipient_address, f.jobs.job1.charge.address);
  const repeatedPrepare = await run(['job', 'prepare-payment', 'job1', '--payer-address', payer, '--policy', f.policyPath], f.env);
  assert.equal(repeatedPrepare.code, 0, repeatedPrepare.stderr);
  assert.equal(repeatedPrepare.json().send_authorized, false);
  const reconciled = await run(['job', 'reconcile', 'job1', '--block-hash', hash], f.env);
  assert.equal(reconciled.json().status, 'confirmed');
  assert.equal(reconciled.json().notification_status, 'blocked');
  const again = await run(['job', 'reconcile', 'job1', '--block-hash', hash], f.env);
  assert.equal(again.code, 0, again.stderr);
  assert.equal(again.json().notification_status, 'notified');
  assert.equal(f.receipts.length, 2);
  assert.equal(f.receipts[0].body, f.receipts[1].body);
  assert.equal(f.receipts[0].key, f.receipts[1].key);
  assert.notEqual(f.receipts[0].nonce, f.receipts[1].nonce);
});

test('crash ambiguity remains reserved through restart and only reconciliation can advance it', async t => {
  const f = await fixture(t, {rejectNotification: true});
  const first = await run(['job', 'prepare-payment', 'job1', '--payer-address', payer, '--policy', f.policyPath], f.env);
  assert.equal(first.json().send_authorized, true);
  const again = await run(['job', 'prepare-payment', 'job1', '--payer-address', payer, '--policy', f.policyPath], f.env);
  assert.equal(again.json().send_authorized, false);
  assert.equal(again.json().status, 'reserved_unknown');
  const reconciled = await run(['job', 'reconcile', 'job1', '--block-hash', hash], f.env);
  assert.equal(reconciled.code, 0, reconciled.stderr);
  assert.equal(reconciled.json().status, 'confirmed');
  assert.equal(reconciled.json().notification_status, 'blocked');
});

test('parallel CLI buyers cannot reserve more than the shared budget', async t => {
  const f = await fixture(t);
  const results = await Promise.all(['job1', 'job2'].map(id => run(['job', 'prepare-payment', id, '--payer-address', payer, '--policy', f.policyPath], f.env)));
  assert.equal(results.filter(r => r.code === 0).length, 1, JSON.stringify(results));
  assert.match(results.find(r => r.code !== 0).stderr, /budget/);
});

test('parallel preparation emits exactly one first handoff', async t => {
  const f = await fixture(t);
  const results = await Promise.all([run(['job', 'prepare-payment', 'job1', '--payer-address', payer, '--policy', f.policyPath], f.env), run(['job', 'prepare-payment', 'job1', '--payer-address', payer, '--policy', f.policyPath], f.env)]);
  assert.deepEqual(results.map(result => result.json().send_authorized).sort(), [false, true]);
});

test('filtered polling cannot ACK omitted events', async t => {
  const f = await fixture(t);
  for (const flags of [['--types', 'job.paid'], ['--since-event-id', '100']]) {
    const result = await run(['poll', ...flags], f.env);
    assert.equal(result.code, 1);
    assert.match(result.stderr, /--no-ack/);
  }
  assert.deepEqual(f.acks, []);
});

test('poll ACK never completes a failed payload task and restarting the CLI retains it', async t => {
  const f = await fixture(t, {events: [{event_id: 42, event_type: 'job.requested', data: {job_id: 'job1', request_payload_id: 'payload1'}}]});
  const poll = await run(['poll'], f.env);
  assert.equal(poll.code, 0, poll.stderr);
  assert.deepEqual(f.acks, [{up_to_event_id: 42}]);
  const queued = await run(['queue', 'list'], f.env);
  assert.equal(queued.json().queue[0].status, 'pending');
  assert.match(queued.json().queue[0].payload_error, /503/);
  const complete = await run(['queue', 'complete', '42'], f.env);
  assert.equal(complete.code, 1);
  const retry = await run(['queue', 'retry'], f.env);
  assert.equal(retry.json().queue[0].status, 'pending');
});
