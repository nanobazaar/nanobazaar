'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const {
  applyEventToKnownState,
  resolveJobPlaybookPath,
  resolveOfferPlaybookPath,
  syncOperatorPlaybooks,
  writeJobPlaybook,
  writeOfferPlaybook,
} = require('../bin/nanobazaar');

test('writeOfferPlaybook creates markdown and preserves operator notes', () => {
  const workspaceDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nanobazaar-offer-playbook-'));

  try {
    const firstPath = writeOfferPlaybook({
      workspaceDir,
      offer: {
        offer_id: 'offer_alpha',
        title: 'Alpha Offer',
        status: 'ACTIVE',
        tags: ['writing', 'nano'],
        price_raw: '1000000000000000000000000000000',
        request_schema_hint: 'Please include a topic.',
        last_updated_at: '2026-03-20T08:00:00Z',
      },
    });
    assert.equal(firstPath, resolveOfferPlaybookPath('offer_alpha', workspaceDir));
    assert.match(fs.readFileSync(firstPath, 'utf8'), /# Offer offer_alpha/);
    assert.match(fs.readFileSync(firstPath, 'utf8'), /price_xno: 1/);

    fs.writeFileSync(firstPath, `${fs.readFileSync(firstPath, 'utf8').trimEnd()}\ncustom note\n`);

    writeOfferPlaybook({
      workspaceDir,
      offer: {
        offer_id: 'offer_alpha',
        title: 'Alpha Offer Updated',
        status: 'PAUSED',
        tags: ['writing'],
        price_raw: '2000000000000000000000000000000',
        request_schema_hint: 'Please include a topic and tone.',
        last_updated_at: '2026-03-20T09:00:00Z',
      },
    });

    const text = fs.readFileSync(firstPath, 'utf8');
    assert.match(text, /title: Alpha Offer Updated/);
    assert.match(text, /status: PAUSED/);
    assert.match(text, /custom note/);
  } finally {
    fs.rmSync(workspaceDir, {recursive: true, force: true});
  }
});

test('applyEventToKnownState derives job state and syncOperatorPlaybooks writes job markdown', () => {
  const workspaceDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nanobazaar-job-playbook-'));
  const statePath = path.join(workspaceDir, 'state.json');
  const state = {
    known_payloads: {
      pay_req: {
        payload_id: 'pay_req',
        job_id: 'job_123',
        payload_kind: 'request',
        cached_path: path.join(workspaceDir, 'payload.json'),
      },
    },
  };

  fs.writeFileSync(state.known_payloads.pay_req.cached_path, JSON.stringify({body: 'Summarize this paper.'}));

  try {
    applyEventToKnownState(state, {
      event_type: 'job.requested',
      data: {
        job_id: 'job_123',
        offer_id: 'offer_123',
        buyer_bot_id: 'buyer_bot',
        seller_bot_id: 'seller_bot',
        price_raw: '3000000000000000000000000000000',
        request_payload_id: 'pay_req',
      },
    }, {now: '2026-03-20T08:10:00Z'});

    applyEventToKnownState(state, {
      event_type: 'job.charge_created',
      data: {
        job_id: 'job_123',
        charge_id: 'chg_123',
        address: 'nano_abc',
        amount_raw: '3000000000000000000000000000000',
        charge_expires_at: '2026-03-20T09:10:00Z',
        charge_sig_ed25519: 'sig',
      },
    }, {now: '2026-03-20T08:11:00Z'});

    applyEventToKnownState(state, {
      event_type: 'job.paid',
      data: {
        job_id: 'job_123',
        verifier: 'berrypay',
        payment_block_hash: 'block_123',
        observed_at: '2026-03-20T08:12:00Z',
        amount_raw_received: '3000000000000000000000000000000',
      },
    }, {now: '2026-03-20T08:12:00Z'});

    syncOperatorPlaybooks({
      state,
      statePath,
      workspaceDir,
      jobIds: ['job_123'],
    });

    const playbookPath = resolveJobPlaybookPath('job_123', workspaceDir);
    const text = fs.readFileSync(playbookPath, 'utf8');

    assert.match(text, /# Job job_123/);
    assert.match(text, /request_payload_summary:\n  Summarize this paper\./);
    assert.match(text, /charge_id: chg_123/);
    assert.match(text, /payment_status: CONFIRMED/);
    assert.match(text, /payment_block_hash/);
    assert.match(text, /- 2026-03-20T08:10:00Z requested/);
    assert.match(text, /- 2026-03-20T08:11:00Z charge created/);
    assert.match(text, /- 2026-03-20T08:12:00Z paid/);
    assert.ok(fs.existsSync(statePath));
  } finally {
    fs.rmSync(workspaceDir, {recursive: true, force: true});
  }
});

test('writeJobPlaybook returns deterministic path', () => {
  const workspaceDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nanobazaar-job-path-'));

  try {
    const playbookPath = writeJobPlaybook({
      workspaceDir,
      job: {
        job_id: 'job_path',
        status_timeline: [],
        last_updated_at: '2026-03-20T08:00:00Z',
      },
    });

    assert.equal(playbookPath, resolveJobPlaybookPath('job_path', workspaceDir));
    assert.ok(fs.existsSync(playbookPath));
  } finally {
    fs.rmSync(workspaceDir, {recursive: true, force: true});
  }
});
