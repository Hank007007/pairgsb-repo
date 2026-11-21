'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { stamp, stampAll } = require('../src/clock');

test('stamping returns a record with the same seq', () => {
  const out = stamp({ seq: 7, ts: 1000, payload: { a: 1 } });
  assert.equal(out.seq, 7);
  assert.deepEqual(out.payload, { a: 1 });
});

test('stamping a list keeps the list length and order', () => {
  const out = stampAll([
    { seq: 1, ts: 1000, payload: {} },
    { seq: 2, ts: 2000, payload: {} },
  ]);
  assert.equal(out.length, 2);
  assert.deepEqual(
    out.map((r) => r.seq),
    [1, 2],
  );
});
