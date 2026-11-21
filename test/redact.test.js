'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { scrub, scrubBatch } = require('../src/redact');

test('a plain message is left unchanged', () => {
  const event = {
    seq: 1,
    ts: 1000,
    payload: { message: 'user logged in' },
    meta: { host: 'node-a' },
  };
  const out = scrub(event);
  assert.deepEqual(out, event);
});

test('redacting a batch keeps every record', () => {
  const batch = {
    batchId: 'b-1',
    records: [
      { seq: 1, payload: { message: 'first' } },
      { seq: 2, payload: { message: 'nothing here' } },
    ],
  };
  const out = scrubBatch(batch);
  assert.equal(out.records.length, 2);
  assert.equal(out.records[0].payload.message, 'first');
  assert.equal(out.records[1].payload.message, 'nothing here');
});

test('redaction leaves the source event untouched', () => {
  const event = { seq: 1, payload: { message: 'first' } };
  scrub(event);
  assert.equal(event.payload.message, 'first');
});
