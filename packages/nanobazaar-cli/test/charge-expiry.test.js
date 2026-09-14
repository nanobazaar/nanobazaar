'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const {formatRFC3339Nano, canonicalChargeString} = require('../bin/nanobazaar');

test('charge expiry uses Go-compatible minimal fractional seconds', () => {
  for (const [input, expected] of [
    ['2026-09-13T12:00:00.000Z', '2026-09-13T12:00:00Z'],
    ['2026-09-13T12:00:00.100Z', '2026-09-13T12:00:00.1Z'],
    ['2026-09-13T12:00:00.120Z', '2026-09-13T12:00:00.12Z'],
    ['2026-09-13T12:00:00.010Z', '2026-09-13T12:00:00.01Z'],
    ['2026-09-13T12:00:00.001Z', '2026-09-13T12:00:00.001Z'],
    ['2026-09-13T12:00:00.123Z', '2026-09-13T12:00:00.123Z'],
  ]) {
    assert.equal(formatRFC3339Nano(new Date(input)), expected, input);
    assert.equal(canonicalChargeString({jobId: 'job', offerId: 'offer', sellerBotId: 'seller', buyerBotId: 'buyer', chargeId: 'charge', address: 'address', amountRaw: '1', chargeExpiresAt: input}),
      `NBR1_CHARGE|job|offer|seller|buyer|charge|address|1|${expected}`);
  }
});

test('all millisecond values preserve their instant in canonical form', () => {
  const base = Date.parse('2026-09-13T12:00:00Z');
  for (let ms = 0; ms < 1000; ms += 1) {
    const actual = formatRFC3339Nano(new Date(base + ms));
    assert.equal(Date.parse(actual), base + ms);
    if (ms === 0) assert.equal(actual, '2026-09-13T12:00:00Z');
    else assert.match(actual, /^2026-09-13T12:00:00\.\d{0,2}[1-9]Z$/, `milliseconds=${ms}`);
  }
});

test('invalid dates remain rejected', () => {
  assert.throws(() => formatRFC3339Nano(new Date(NaN)), /Invalid date/);
});
