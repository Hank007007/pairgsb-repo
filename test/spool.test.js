'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { Spool, QUEUE_FILE, CURSOR_FILE } = require('../src/spool');

function tempDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'sealferry-spool-'));
}

test('appended records are queued and survive a reload', () => {
  const dir = tempDir();
  const spool = new Spool(dir, { maxRecords: 10 });
  spool.load();
  spool.saveCursor(120, [
    { seq: 1, ts: 10, prev_hash: null, payload: { a: 1 }, meta: null },
    { seq: 2, ts: 11, prev_hash: 'h', payload: { a: 2 }, meta: null },
  ]);

  assert.equal(spool.pending().length, 2);
  assert.equal(spool.getCursor(), 120);

  const reopened = new Spool(dir, { maxRecords: 10 });
  reopened.load();

  assert.equal(reopened.getCursor(), 120);
  assert.deepEqual(
    reopened.pending().map((r) => r.seq),
    [1, 2],
  );
});

test('queue records are stored one JSON object per line', () => {
  const dir = tempDir();
  const spool = new Spool(dir, { maxRecords: 10 });
  spool.load();
  spool.append({ seq: 1, ts: 1, payload: {} });
  spool.append({ seq: 2, ts: 2, payload: {} });

  const lines = fs.readFileSync(path.join(dir, QUEUE_FILE), 'utf8').trim().split('\n');
  assert.equal(lines.length, 2);
  assert.deepEqual(JSON.parse(lines[1]), { seq: 2, ts: 2, payload: {} });
});

test('capacity keeps the newest records', () => {
  const dir = tempDir();
  const spool = new Spool(dir, { maxRecords: 3 });
  spool.load();
  for (let seq = 1; seq <= 5; seq += 1) {
    spool.append({ seq, ts: seq, payload: {} });
  }
  assert.deepEqual(
    spool.pending().map((r) => r.seq),
    [3, 4, 5],
  );
});

test('delivered records leave the queue and the queue file', () => {
  const dir = tempDir();
  const spool = new Spool(dir, { maxRecords: 10 });
  spool.load();
  const a = { seq: 1, ts: 1, payload: {} };
  const b = { seq: 2, ts: 2, payload: {} };
  spool.append(a);
  spool.append(b);

  spool.markDelivered([a]);

  assert.deepEqual(spool.pending(), [b]);
  const reopened = new Spool(dir, { maxRecords: 10 });
  reopened.load();
  assert.deepEqual(reopened.pending(), [b]);
});

test('a fresh spool starts at offset zero with an empty queue', () => {
  const dir = tempDir();
  const spool = new Spool(dir, { maxRecords: 10 });
  spool.load();

  assert.equal(spool.getCursor(), 0);
  assert.deepEqual(spool.pending(), []);
  assert.equal(fs.existsSync(path.join(dir, CURSOR_FILE)), false);
});
