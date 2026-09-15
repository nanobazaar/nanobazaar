'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const http = require('node:http');
const {spawn} = require('node:child_process');

test('complete CLI trade through real authenticated relay, migrations and encrypted payloads', {skip: !process.env.NBR_TEST_RELAY_BIN, timeout: 60000}, async t => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'nbr-real-relay-'));
  const cli = process.env.NBR_TEST_CLI_BIN || path.resolve(__dirname, '../bin/nanobazaar');
  const payer = 'nano_1' + '1'.repeat(59), address = 'nano_3' + '1'.repeat(59), hash = 'B'.repeat(64), amount = '100000000000000000000000000';
  const rpc = http.createServer(async (req, res) => {
    let body = ''; for await (const chunk of req) body += chunk;
    res.setHeader('Content-Type', 'application/json');
    const query = JSON.parse(body);
    const result = query.action === 'receivable' ? {blocks: ''}
      : query.action === 'account_info' ? query.account === address ? {error: 'Account not found'} : {block_count: '4'}
      : {confirmed: 'true', subtype: 'send', block_account: payer, height: '5', amount, contents: {account: payer, link_as_account: address}};
    res.end(JSON.stringify(result));
  });
  await new Promise(resolve => rpc.listen(0, '127.0.0.1', resolve));
  const portProbe = http.createServer();
  await new Promise(resolve => portProbe.listen(0, '127.0.0.1', resolve));
  const port = portProbe.address().port;
  await new Promise(resolve => portProbe.close(resolve));
  const base = `http://127.0.0.1:${port}`;
  const relay = spawn(process.env.NBR_TEST_RELAY_BIN, [], {cwd: path.resolve(__dirname, '../../../apps/relay'), env: {...process.env, NBR_HTTP_ADDR: `127.0.0.1:${port}`, NBR_DB_PATH: path.join(dir, 'relay.db'), NBR_MIGRATE_ON_START: 'true', NBR_RETENTION_ENABLED: 'false', NBR_ADMIN_ADDR: '', NBR_ADMIN_PUBLIC: 'false', NBR_METRICS_ADDR: '', NBR_RL_WRITES_BURST: '100', NBR_RL_POLL_BURST: '100'}});
  let logs = '';
  relay.stderr.on('data', b => { logs += b; });
  t.after(async () => {
    const closed = new Promise(resolve => relay.once('close', resolve));
    if (relay.exitCode === null) { relay.kill('SIGTERM'); await closed; }
    rpc.closeAllConnections(); await new Promise(resolve => rpc.close(resolve));
    fs.rmSync(dir, {recursive: true, force: true});
  });
  for (let i = 0; i < 100 && !logs.includes('relay listening'); i++) await new Promise(resolve => setTimeout(resolve, 30));
  assert.match(logs, /relay listening/, logs);
  async function command(role, args, {allowError = false} = {}) {
    const env = {...process.env, NBR_RELAY_URL: base, NBR_STATE_PATH: path.join(dir, role + '.json'), NBR_NANO_RPC_URL: `http://127.0.0.1:${rpc.address().port}`, NBR_IDEMPOTENCY_KEY: '', NBR_SIGNING_PRIVATE_KEY_B64URL: '', NBR_SIGNING_PUBLIC_KEY_B64URL: '', NBR_ENCRYPTION_PRIVATE_KEY_B64URL: '', NBR_ENCRYPTION_PUBLIC_KEY_B64URL: ''};
    return new Promise((resolve, reject) => {
      const child = process.env.NBR_TEST_CLI_BIN
        ? spawn(cli, args, {env})
        : spawn(process.execPath, [cli, ...args], {env});
      let out = '', err = '';
      child.stdout.on('data', b => { out += b; }); child.stderr.on('data', b => { err += b; });
      child.on('close', code => {
        if (code !== 0 && !allowError) return reject(new Error(`${role} ${args.join(' ')}: ${err}\n${logs}`));
        resolve({code, out, err, json: () => JSON.parse(out)});
      });
    });
  }
  await command('seller', ['setup']);
  await command('buyer', ['setup']);
  const seller = JSON.parse(fs.readFileSync(path.join(dir, 'seller.json'))).bot_id;
  const buyer = JSON.parse(fs.readFileSync(path.join(dir, 'buyer.json'))).bot_id;
  const offer = (await command('seller', ['offer', 'create', '--title', 'Fixture service', '--description', 'Local test only', '--tag', 'test', '--price-raw', amount, '--turnaround-seconds', '60'])).json();
  const search = (await command('buyer', ['search', 'Fixture'])).json();
  assert.ok(search.offers.some(item => item.offer_id === offer.offer_id));
  const job = (await command('buyer', ['job', 'create', '--offer-id', offer.offer_id, '--request-body', 'A deterministic fixture request'])).json();
  await command('seller', ['poll']);
  const queued = (await command('seller', ['queue', 'list'])).json().queue;
  assert.ok(queued.some(item => item.payload_ready));
  await command('seller', ['job', 'charge', job.job_id, '--address', address, '--no-qr']);
  const policy = path.join(dir, 'policy.json');
  fs.writeFileSync(policy, JSON.stringify({buyer_bot_id: buyer, allowed_sellers: [seller], max_payment_raw: amount, total_budget_raw: amount, expires_at: new Date(Date.now() + 3600000).toISOString()}));
  const handoff = (await command('buyer', ['job', 'prepare-payment', job.job_id, '--payer-address', payer, '--policy', policy])).json();
  assert.equal(handoff.send_authorized, true);
  assert.equal(handoff.amount_raw, amount);
  assert.equal(handoff.recipient_address, address);
  const paid = (await command('buyer', ['job', 'reconcile', job.job_id, '--block-hash', hash])).json();
  assert.equal(paid.status, 'confirmed');
  assert.equal(paid.notification_status, 'notified', JSON.stringify(paid));
  const accepted = (await command('seller', ['job', 'accept-payment', job.job_id, '--block-hash', hash, '--payer-address', payer])).json();
  assert.equal(accepted.notification_status, 'notified', JSON.stringify(accepted));
  await command('seller', ['job', 'deliver', job.job_id, '--body', 'Verified local deliverable']);
  // A fresh buyer can poll even when the first recipient event has a high global ID.
  await command('buyer', ['poll']);
  // Explicit recovery remains safe after the normal poll has already persisted work.
  await command('buyer', ['queue', 'resync']);
  await command('buyer', ['queue', 'retry']);
  const buyerQueue = (await command('buyer', ['queue', 'list'])).json().queue;
  assert.ok(buyerQueue.some(item => item.payload_ready));
  const deliveryState = (await command('buyer', ['job', 'get', job.job_id])).json();
  assert.equal(deliveryState.status, 'DELIVERED');
  const payload = (await command('buyer', ['payload', 'fetch', '--job-id', job.job_id])).json();
  assert.equal(payload.body, 'Verified local deliverable');
  const repeated = (await command('buyer', ['job', 'prepare-payment', job.job_id, '--payer-address', payer, '--policy', policy])).json();
  assert.equal(repeated.send_authorized, false);
  const repeatedDelivery = await command('seller', ['job', 'deliver', job.job_id, '--body', 'Verified local deliverable'], {allowError: true});
  assert.equal(repeatedDelivery.code, 1);
  assert.match(repeatedDelivery.err, /outbox retry/);
  const outbox = (await command('seller', ['outbox', 'list', '--all'])).json().outbox;
  const delivery = outbox.find(item => item.path.endsWith('/deliver'));
  assert.ok(delivery);
  await command('seller', ['outbox', 'retry', delivery.id]);
});
