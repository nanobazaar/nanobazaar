'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {Journal, readJson, writeJson} = require('../lib/journal');

function fixture(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'nbr-journal-'));
  t.after(() => fs.rmSync(dir, {recursive: true, force: true}));
  return path.join(dir, 'state.json');
}

test('missing state is empty; corrupt or non-object state is never silently replaced', t => {
  const file = fixture(t);
  assert.deepEqual(readJson(file), {});
  for (const text of ['{', 'null', '[]']) {
    fs.writeFileSync(file, text);
    assert.throws(() => readJson(file), /state|JSON/i);
    assert.equal(fs.readFileSync(file, 'utf8'), text);
  }
});

test('atomic JSON files are private and journal scope cannot change', t => {
  const file = fixture(t);
  writeJson(file, {secret: 'fixture'});
  assert.equal(fs.statSync(file).mode & 0o777, 0o600);
  const j = new Journal(file + '.ops', 'relay|bot');
  j.transaction(s => { s.counter = 1; });
  assert.equal(j.read().counter, 1);
  assert.throws(() => new Journal(file + '.ops', 'relay|other').read(), /scope/);
  fs.writeFileSync(file + '.ops', '{}');
  assert.throws(() => j.read(), /version|scope/);
});

test('outbox persists exact bytes and rejects changed requests under the same key', t => {
  const j = new Journal(fixture(t), 'relay|bot');
  const request = {method: 'POST', path: '/v0/jobs', body: '{ "payload": "ciphertext" }', idempotencyKey: 'job-1'};
  const first = j.prepareRequest(request);
  assert.equal(j.prepareRequest(request).id, first.id);
  assert.equal(j.read().outbox[first.id].body, request.body);
  assert.throws(() => j.prepareRequest({...request, body: '{"payload":"different"}'}), /outbox retry/);
});

test('malformed optional seller maps fail before any receipt can disappear during serialization', t => {
  const file = fixture(t);
  const j = new Journal(file, 'relay|bot');
  const valid = j.read();
  for (const field of ['seller_receipts', 'seller_charges', 'charge_addresses']) {
    for (const value of [[], null, 'invalid']) {
      writeJson(file, {...valid, [field]: value});
      assert.throws(() => j.transaction(data => { data[field].job1 = {status: 'confirmed'}; }), new RegExp(`Invalid journal ${field}`));
    }
  }
});

test('unfinished work outlives display history and completed work does not reopen', t => {
  const j = new Journal(fixture(t), 'relay|bot');
  j.ingest(Array.from({length: 601}, (_, i) => ({event_id: i + 1, event_type: 'job.created'})));
  assert.equal(Object.keys(j.read().queue).length, 601);
  j.complete('1');
  j.ingest([{event_id: 1, event_type: 'job.created'}]);
  assert.equal(j.read().queue['1'].status, 'done');
  assert.equal(j.read().queue['601'].status, 'pending');
});

test('duplicate event ingestion does not rewrite the journal', t => {
  const file = fixture(t);
  const j = new Journal(file, 'relay|bot');
  const events = [{event_id: 1, event_type: 'job.created'}];
  j.ingest(events);
  const inode = fs.statSync(file).ino;
  j.ingest(events);
  j.ingest([]);
  assert.equal(fs.statSync(file).ino, inode);
});
