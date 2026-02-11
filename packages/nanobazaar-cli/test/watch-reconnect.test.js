'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const {
  backoffDelayMs,
  planReconnect,
  WATCH_RECONNECT_BASE_MS,
  WATCH_RECONNECT_FACTOR,
  WATCH_RECONNECT_CAP_MS,
  WATCH_RECONNECT_JITTER_RATIO,
  WATCH_RECONNECT_STABLE_WINDOW_MS,
} = require('../bin/nanobazaar');

test('backoff defaults use 1s base, 2x factor, and 60s cap', () => {
  assert.equal(backoffDelayMs(0, {random: () => 0.5}), WATCH_RECONNECT_BASE_MS);
  assert.equal(backoffDelayMs(1, {random: () => 0.5}), 2000);
  assert.equal(backoffDelayMs(2, {random: () => 0.5}), 4000);
  assert.equal(backoffDelayMs(10, {random: () => 0.5}), WATCH_RECONNECT_CAP_MS);
  assert.equal(WATCH_RECONNECT_FACTOR, 2);
});

test('backoff jitter stays within configured jitter band', () => {
  const attempt = 3;
  const exp = WATCH_RECONNECT_BASE_MS * (WATCH_RECONNECT_FACTOR ** attempt);
  const low = Math.round(exp * (1 - WATCH_RECONNECT_JITTER_RATIO));
  const high = Math.round(exp * (1 + WATCH_RECONNECT_JITTER_RATIO));

  assert.equal(backoffDelayMs(attempt, {random: () => 0}), low);
  assert.equal(backoffDelayMs(attempt, {random: () => 1}), high);
});

test('reconnect plan resets backoff after stable connection window', () => {
  const plan = planReconnect(4, {
    reason: 'SSE stream disconnected',
    connectedDurationMs: WATCH_RECONNECT_STABLE_WINDOW_MS + 1,
    backoff: {random: () => 0.5},
  });

  assert.equal(plan.reset, true);
  assert.equal(plan.attemptNumber, 1);
  assert.equal(plan.nextAttempt, 1);
  assert.equal(plan.delayMs, WATCH_RECONNECT_BASE_MS);
  assert.equal(plan.connectedDurationMs, WATCH_RECONNECT_STABLE_WINDOW_MS + 1);
});

test('reconnect plan increments attempt without stable duration', () => {
  const plan = planReconnect(2, {
    reason: 'SSE connect failed',
    backoff: {random: () => 0.5},
  });

  assert.equal(plan.reset, false);
  assert.equal(plan.attemptNumber, 3);
  assert.equal(plan.nextAttempt, 3);
  assert.equal(plan.delayMs, 4000);
  assert.equal(plan.connectedDurationMs, null);
});
