import test from 'node:test';
import assert from 'node:assert/strict';

import { isRetryableStatus, computeDelay, createRetryQueue } from '../src/retry.js';

test('isRetryableStatus covers 429 and the transient 5xx family', () => {
  for (const status of [429, 500, 502, 503, 504]) {
    assert.equal(isRetryableStatus(status), true);
  }
  for (const status of [200, 400, 401, 403, 404, 501, 505]) {
    assert.equal(isRetryableStatus(status), false);
  }
});

test('computeDelay grows exponentially and clamps at maxDelayMs', () => {
  const noJitter = (attempt) => computeDelay({ attempt, baseDelayMs: 100, maxDelayMs: 1000, jitter: 0 });
  assert.equal(noJitter(1), 100);
  assert.equal(noJitter(2), 200);
  assert.equal(noJitter(3), 400);
  assert.equal(noJitter(5), 1000);
  assert.equal(noJitter(9), 1000);
});

test('computeDelay applies jitter within bounds', () => {
  const low = computeDelay({ attempt: 3, baseDelayMs: 100, maxDelayMs: 5000, jitter: 0.25, random: () => 0 });
  const high = computeDelay({ attempt: 3, baseDelayMs: 100, maxDelayMs: 5000, jitter: 0.25, random: () => 1 });
  assert.equal(low, 300);
  assert.equal(high, 500);
});

test('a queued job runs after its delay', async () => {
  const queue = createRetryQueue({ baseDelayMs: 5, maxDelayMs: 5, jitter: 0 });
  const received = [];
  const handle = queue.enqueue({
    url: 'https://api.partner.com/v1',
    method: 'GET',
    body: null,
    headers: { Accept: 'application/json' },
    entry: null,
    attempt: 2,
    dispatch: async (request) => {
      received.push(request);
    },
  });

  assert.equal(queue.size(), 1);
  assert.ok(handle.delayMs >= 0);
  await waitFor(() => received.length === 1, 1000);
  assert.equal(queue.size(), 0);
  assert.equal(received[0].url, 'https://api.partner.com/v1');
  assert.equal(received[0].attempt, 2);
  queue.close();
});

test('cancel prevents a queued job from running', async () => {
  const queue = createRetryQueue({ baseDelayMs: 20, maxDelayMs: 20, jitter: 0 });
  let ran = false;
  const handle = queue.enqueue({
    url: 'https://api.partner.com/v1',
    method: 'GET',
    body: null,
    headers: {},
    entry: null,
    attempt: 2,
    dispatch: async () => {
      ran = true;
    },
  });

  assert.equal(handle.cancel(), true);
  assert.equal(handle.cancel(), false);
  assert.equal(queue.size(), 0);
  await sleep(60);
  assert.equal(ran, false);
  queue.close();
});

test('closing the queue cancels pending jobs', async () => {
  const queue = createRetryQueue({ baseDelayMs: 20, maxDelayMs: 20, jitter: 0 });
  const reasons = [];
  queue.enqueue({
    url: 'https://api.partner.com/v1',
    method: 'GET',
    body: null,
    headers: {},
    entry: null,
    attempt: 2,
    dispatch: async () => {},
    onCancel: (reason) => reasons.push(reason),
  });
  queue.close();
  assert.equal(queue.size(), 0);
  assert.deepEqual(reasons, ['queue-closed']);
});

test('a dispatch failure is routed to onError', async () => {
  const queue = createRetryQueue({ baseDelayMs: 5, maxDelayMs: 5, jitter: 0 });
  const errors = [];
  queue.enqueue({
    url: 'https://api.partner.com/v1',
    method: 'GET',
    body: null,
    headers: {},
    entry: null,
    attempt: 2,
    dispatch: async () => {
      throw new Error('dispatch failed');
    },
    onError: (error) => errors.push(error),
  });
  await waitFor(() => errors.length === 1, 1000);
  assert.equal(errors[0].message, 'dispatch failed');
  queue.close();
});

test('enqueueing on a closed queue throws', () => {
  const queue = createRetryQueue();
  queue.close();
  assert.throws(() => queue.enqueue({ url: 'x', method: 'GET', headers: {}, attempt: 1, dispatch: async () => {} }));
});

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitFor(predicate, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (!predicate()) {
    if (Date.now() > deadline) {
      throw new Error('timed out waiting for condition');
    }
    await sleep(5);
  }
}
