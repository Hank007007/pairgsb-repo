'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { send, deliverOnce } = require('../src/deliver');

function batch(overrides = {}) {
  return {
    batchId: 'b-1',
    source: 'test',
    prevDigest: null,
    records: [{ seq: 1, ts: 1000, payload: { a: 1 } }],
    digest: 'a'.repeat(64),
    ...overrides,
  };
}

function transport(handler) {
  return { post: handler };
}

test('a matching acknowledgement is reported as success', async () => {
  const sent = [];
  const result = await send(
    batch(),
    transport(async (b) => {
      sent.push(b);
      return { batchId: b.batchId, digest: b.digest };
    }),
    { attempts: 3, backoffMs: 0 },
  );

  assert.equal(result.ok, true);
  assert.equal(sent.length, 1);
});

test('a transport failure followed by success resolves as success', async () => {
  let calls = 0;
  const result = await send(
    batch(),
    transport(async (b) => {
      calls += 1;
      if (calls === 1) {
        throw new Error('network down');
      }
      return { batchId: b.batchId, digest: b.digest };
    }),
    { attempts: 3, backoffMs: 0 },
  );

  assert.equal(result.ok, true);
  assert.equal(calls, 2);
});

test('a retry sends the identical batch', async () => {
  const seen = [];
  const original = batch();
  await send(
    original,
    transport(async (b) => {
      seen.push(b);
      if (seen.length < 2) {
        throw new Error('network down');
      }
      return { batchId: b.batchId, digest: b.digest };
    }),
    { attempts: 3, backoffMs: 0 },
  );

  assert.equal(seen.length, 2);
  assert.deepEqual(seen[1], seen[0]);
});

test('deliverOnce reports the delivery outcome', async () => {
  const result = await deliverOnce(
    batch(),
    transport(async (b) => ({ batchId: b.batchId, digest: b.digest })),
    { attempts: 1, backoffMs: 0 },
  );
  assert.equal(result.ok, true);
});
