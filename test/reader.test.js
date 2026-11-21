'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { SourceReader } = require('../src/reader');

function tempDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'sealferry-reader-'));
}

test('reads complete lines and parses the required fields', () => {
  const dir = tempDir();
  const file = path.join(dir, 'events.log');
  fs.writeFileSync(
    file,
    [
      JSON.stringify({ seq: 1, ts: 1700000000000, prev_hash: null, payload: { a: 1 } }),
      JSON.stringify({ seq: 2, ts: 1700000000001, prev_hash: 'aa', payload: { a: 2 } }),
      '',
    ].join('\n'),
    'utf8',
  );

  const reader = new SourceReader(file);
  const result = reader.readFrom(0);

  assert.equal(result.events.length, 2);
  assert.equal(result.events[0].seq, 1);
  assert.equal(result.events[0].ts, 1700000000000);
  assert.deepEqual(result.events[1].payload, { a: 2 });
  assert.equal(result.pending, '');
});

test('a trailing fragment without a terminator is not consumed', () => {
  const dir = tempDir();
  const file = path.join(dir, 'events.log');
  const complete = `${JSON.stringify({ seq: 1, ts: 1, payload: { a: 1 } })}\n`;
  fs.writeFileSync(file, `${complete}{"seq":2,"ts":2,"pay`, 'utf8');

  const reader = new SourceReader(file);
  const result = reader.readFrom(0);

  assert.equal(result.events.length, 1);
  assert.equal(result.offset, Buffer.byteLength(complete, 'utf8'));
  assert.equal(result.pending, '{"seq":2,"ts":2,"pay');
});

test('reading from an offset returns only new lines', () => {
  const dir = tempDir();
  const file = path.join(dir, 'events.log');
  const first = `${JSON.stringify({ seq: 1, ts: 1, payload: { a: 1 } })}\n`;
  fs.writeFileSync(file, first, 'utf8');

  const reader = new SourceReader(file);
  const initial = reader.readFrom(0);
  assert.equal(initial.events.length, 1);

  fs.appendFileSync(file, `${JSON.stringify({ seq: 2, ts: 2, payload: { a: 2 } })}\n`);
  const next = reader.readFrom(initial.offset);
  assert.equal(next.events.length, 1);
  assert.equal(next.events[0].seq, 2);
});

test('a missing source file yields no events', () => {
  const reader = new SourceReader(path.join(tempDir(), 'absent.log'));
  const result = reader.readFrom(0);
  assert.deepEqual(result.events, []);
  assert.equal(result.offset, 0);
});
