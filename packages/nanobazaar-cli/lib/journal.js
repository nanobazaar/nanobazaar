'use strict';

const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

function readJson(file) {
  try {
    const value = JSON.parse(fs.readFileSync(file, 'utf8'));
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Expected a JSON object');
    return value;
  } catch (err) {
    if (err.code === 'ENOENT') return {};
    throw new Error(`Cannot read state ${file}: ${err.message}. Restore it from a trusted backup; do not reset payment history.`);
  }
}

function writeJson(file, value, indent = 2) {
  fs.mkdirSync(path.dirname(file), {recursive: true, mode: 0o700});
  const tmp = `${file}.${process.pid}.${crypto.randomBytes(8).toString('hex')}.tmp`;
  let fd;
  try {
    fd = fs.openSync(tmp, 'wx', 0o600);
    fs.writeFileSync(fd, JSON.stringify(value, null, indent));
    fs.fsyncSync(fd);
    fs.closeSync(fd);
    fd = undefined;
    fs.renameSync(tmp, file);
    const dir = fs.openSync(path.dirname(file), 'r');
    try { fs.fsyncSync(dir); } finally { fs.closeSync(dir); }
  } finally {
    if (fd !== undefined) fs.closeSync(fd);
    try { fs.unlinkSync(tmp); } catch (err) { if (err.code !== 'ENOENT') throw err; }
  }
}

// Short synchronous transactions only. Never steal a lock on a timer: an old
// process may still be alive and about to persist a payment reservation.
function locked(file, fn) {
  fs.mkdirSync(path.dirname(file), {recursive: true, mode: 0o700});
  const lock = `${file}.lock`;
  const end = Date.now() + 5000;
  let fd;
  while (fd === undefined) {
    try { fd = fs.openSync(lock, 'wx', 0o600); }
    catch (err) {
      if (err.code !== 'EEXIST') throw err;
      if (Date.now() >= end) throw new Error(`State lock busy: ${lock}. If its owner crashed, stop all NanoBazaar processes and remove only this .lock file; keep the journal.`);
      Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 25);
    }
  }
  try {
    fs.writeFileSync(fd, `${process.pid}\n`);
    return fn();
  } finally {
    fs.closeSync(fd);
    fs.unlinkSync(lock);
  }
}

class Journal {
  constructor(file, scope) { this.file = file; this.scope = scope; }

  read() {
    const data = readJson(this.file);
    if (!Object.keys(data).length && !fs.existsSync(this.file)) {
      return {version: 1, scope: this.scope, outbox: {}, queue: {}, payments: {}};
    }
    if (data.version !== 1 || data.scope !== this.scope) throw new Error('Journal version/scope mismatch. Use the original relay and bot.');
    for (const field of ['outbox', 'queue', 'payments']) {
      if (!data[field] || typeof data[field] !== 'object' || Array.isArray(data[field])) throw new Error(`Invalid journal ${field}`);
    }
    for (const field of ['seller_receipts', 'seller_charges', 'charge_addresses']) {
      if (Object.hasOwn(data, field) && (!data[field] || typeof data[field] !== 'object' || Array.isArray(data[field]))) throw new Error(`Invalid journal ${field}`);
    }
    return data;
  }

  transaction(fn) {
    return locked(this.file, () => {
      const data = this.read();
      const before = JSON.stringify(data);
      const result = fn(data);
      if (result && typeof result.then === 'function') throw new Error('Journal transactions must be synchronous');
      if (JSON.stringify(data) !== before) writeJson(this.file, data);
      return result;
    });
  }

  prepareRequest(request) {
    const id = crypto.createHash('sha256').update(JSON.stringify([request.method, request.path, request.idempotencyKey])).digest('hex');
    return this.transaction(data => {
      const old = data.outbox[id];
      if (old) {
        if (old.body !== request.body || JSON.stringify(old.query) !== JSON.stringify(request.query)) {
          throw new Error(`Request changed under an existing idempotency key. Use: nanobazaar outbox retry ${id}`);
        }
        return old;
      }
      return data.outbox[id] = {...request, id, status: 'pending', created_at: new Date().toISOString()};
    });
  }

  claimRequest(id) {
    return this.transaction(data => {
      const item = data.outbox[id];
      if (item.owner_pid) {
        let alive = true;
        try { process.kill(item.owner_pid, 0); } catch (err) { if (err.code === 'ESRCH') alive = false; }
        if (alive) throw new Error('This outbox request is already running. Wait for its process to finish before retrying.');
      }
      item.owner_pid = process.pid;
    });
  }

  releaseRequest(id) {
    this.transaction(data => { delete data.outbox[id].owner_pid; });
  }

  finishRequest(id, response) {
    this.transaction(data => {
      const item = data.outbox[id];
      item.status = response.ok ? 'done' : 'failed';
      item.http_status = response.status;
      item.updated_at = new Date().toISOString();
    });
  }

  ingest(events) {
    this.transaction(data => {
      for (const event of events) {
        if (!Number.isSafeInteger(event.event_id) || event.event_id <= 0) throw new Error('Invalid event_id');
        const id = String(event.event_id);
        if (!Object.hasOwn(data.queue, id)) data.queue[id] = {event, status: 'pending', received_at: new Date().toISOString()};
      }
    });
  }

  complete(id) {
    this.transaction(data => {
      if (!Object.hasOwn(data.queue, id)) throw new Error('Unknown queue event');
      const item = data.queue[id];
      if (item.payload_error) throw new Error('Payload retrieval failed. Run queue retry before completing the event.');
      item.status = 'done';
      item.completed_at = new Date().toISOString();
    });
  }
}

module.exports = {Journal, readJson, writeJson, locked};
