import assert from 'node:assert/strict';
import test from 'node:test';
import { getRelayStats, parseDemandStats } from '../lib/relay-stats.ts';

const demand = {
  window_days: 28,
  window_start: '2026-09-09T19:00:00.123456789Z',
  window_end: '2026-10-07T19:00:00.123456789Z',
  paid_jobs: 5, delivered_jobs: 2, unique_buyers: 3, repeat_buyers: 1
};

test('demand maps only aggregate fields and preserves exact window metadata', () => {
  assert.deepEqual(parseDemandStats({ ...demand, buyer_bot_id: 'must-not-propagate' }), {
    windowDays: 28, windowStart: demand.window_start, windowEnd: demand.window_end,
    paidJobs: 5, deliveredJobs: 2, uniqueBuyers: 3, repeatBuyers: 1
  });
});

test('genuine zero demand is different from unavailable demand', () => {
  assert.equal(parseDemandStats({ ...demand, paid_jobs: 0, delivered_jobs: 0, unique_buyers: 0, repeat_buyers: 0 }).repeatBuyers, 0);
  for (const value of [null, undefined, {}, [], 0, '0']) {
    assert.equal(parseDemandStats(value), null);
  }
  for (const key of Object.keys(demand)) {
    const partial = { ...demand };
    delete partial[key];
    assert.equal(parseDemandStats(partial), null, key);
  }
});

test('invalid counts or inconsistent cohorts never render as trustworthy numbers', () => {
  for (const key of ['paid_jobs', 'delivered_jobs', 'unique_buyers', 'repeat_buyers']) {
    for (const value of [-1, NaN, Infinity, 1.5, '1', null, Number.MAX_SAFE_INTEGER + 1]) {
      assert.equal(parseDemandStats({ ...demand, [key]: value }), null);
    }
  }
  for (const patch of [
    { delivered_jobs: 6 }, { unique_buyers: 6 }, { repeat_buyers: 4 },
    { unique_buyers: 4, repeat_buyers: 2 }, { unique_buyers: 0, repeat_buyers: 0 },
    { window_days: 30 }, { window_start: 'invalidZ' }, { window_end: demand.window_start },
    { window_end: '2026-10-07T19:00:00+00:00' }
  ]) assert.equal(parseDemandStats({ ...demand, ...patch }), null);
});

test('fetch handles old relays, cache settings, zero demand, and failures', async t => {
  const oldURL = process.env.RELAY_STATS_URL;
  const oldPublicURL = process.env.NEXT_PUBLIC_RELAY_STATS_URL;
  t.after(() => {
    if (oldURL === undefined) delete process.env.RELAY_STATS_URL;
    else process.env.RELAY_STATS_URL = oldURL;
    if (oldPublicURL === undefined) delete process.env.NEXT_PUBLIC_RELAY_STATS_URL;
    else process.env.NEXT_PUBLIC_RELAY_STATS_URL = oldPublicURL;
  });
  process.env.RELAY_STATS_URL = 'https://relay.example/stats';
  const legacy = { agents_online: 84, offers: 2, jobs: 40, xno_transferred: '0.05421' };
  let response = legacy;
  const fetch = t.mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(url, process.env.RELAY_STATS_URL);
    assert.deepEqual(options, { next: { revalidate: 60 } });
    return Response.json(response);
  });
  assert.deepEqual(await getRelayStats(), {
    agentsOnline: 84, offers: 2, jobs: 40, xnoTransferred: 0.05421, demand: null
  });
  response = { ...legacy, demand };
  assert.equal((await getRelayStats()).demand.repeatBuyers, 1);
  response = { ...legacy, demand: { ...demand, repeat_buyers: null } };
  assert.equal((await getRelayStats()).demand, null);
  fetch.mock.mockImplementation(async () => new Response('unavailable', { status: 503 }));
  assert.equal(await getRelayStats(), null);
  fetch.mock.mockImplementation(async () => { throw new Error('offline'); });
  assert.equal(await getRelayStats(), null);
  fetch.mock.mockImplementation(async () => new Response('bad json'));
  assert.equal(await getRelayStats(), null);
  delete process.env.RELAY_STATS_URL;
  delete process.env.NEXT_PUBLIC_RELAY_STATS_URL;
  assert.equal(await getRelayStats(), null);
});
