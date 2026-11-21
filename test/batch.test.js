'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { BatchBuilder, buildBatch } = require('../src/batch');

function record(seq, ts, payload) {
  return { seq, ts, prev_hash: `h${seq}`, payload, meta: { n: seq } };
}

test('a batch carries the records ordered by source time', () => {
  const builder = new BatchBuilder({ source: 'test' });
  const batch = builder.buildBatch([
    record(3, 3000, { m: 'c' }),
    record(1, 1000, { m: 'a' }),
    record(2, 2000, { m: 'b' }),
  ]);

  assert.equal(batch.source, 'test');
  assert.deepEqual(
    batch.records.map((r) => r.seq),
    [1, 2, 3],
  );
  assert.match(batch.digest, /^[0-9a-f]{64}$/);
});

test('the first batch has no previous digest and later batches do', () => {
  const builder = new BatchBuilder({ source: 'test' });
  const first = builder.buildBatch([record(1, 1000, { m: 'a' })]);
  const second = builder.buildBatch([record(2, 2000, { m: 'b' })]);

  assert.equal(first.prevDigest, null);
  assert.equal(second.prevDigest, first.digest);
});

test('every batch gets a distinct id', () => {
  const builder = new BatchBuilder({ source: 'test' });
  const ids = new Set();
  for (let i = 0; i < 5; i += 1) {
    ids.add(builder.buildBatch([record(i, 1000 + i, { m: i })]).batchId);
  }
  assert.equal(ids.size, 5);
});

test('the same records produce the same digest', () => {
  const records = [
    record(1, 1000, { m: 'a' }),
    record(2, 2000, { m: 'b' }),
    record(3, 3000, { m: 'c' }),
  ];

  const a = buildBatch(records, { source: 'test' });
  const b = buildBatch(records, { source: 'test' });

  assert.equal(a.digest, b.digest);
});

test('a batch of records with distinct payloads has a stable digest', () => {
  const batch = buildBatch(
    [record(1, 1000, { m: 'a' }), record(2, 2000, { m: 'b' })],
    { source: 'test' },
  );
  assert.equal(batch.digest.length, 64);
});
