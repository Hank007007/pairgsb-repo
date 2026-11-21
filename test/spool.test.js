'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const {
  listStreams,
  readBatch,
  segmentFileName,
} = require('../src/forwarder/spool');

function tempSpool(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'spoolbatch-spool-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

function writeSegment(dir, origin, epoch, index, records) {
  const name = segmentFileName(origin, epoch, index);
  fs.writeFileSync(path.join(dir, name), records.map((r) => `${r}\n`).join(''), 'utf8');
  return name;
}

function cursorAt(segment, offset = 0, nextSeq = 0) {
  return { version: 1, segment, offset, nextSeq, ackedSeq: nextSeq - 1 };
}

test('segmentFileName encodes the origin and pads the index', () => {
  assert.equal(segmentFileName('web-01#a', 2, 7), 'web-01%23a.2.000007.ndjson');
});

test('listStreams groups files by origin and epoch', (t) => {
  const dir = tempSpool(t);
  writeSegment(dir, 'web-01#a', 0, 0, ['{"n":1}']);
  writeSegment(dir, 'web-01#a', 0, 1, ['{"n":2}']);
  writeSegment(dir, 'web-02#b', 0, 0, ['{"n":3}']);

  const streams = listStreams(dir);
  assert.equal(streams.length, 2);
  const first = streams.find((s) => s.origin === 'web-01#a');
  assert.equal(first.files.length, 2);
});

test('readBatch returns whole records from a single segment', (t) => {
  const dir = tempSpool(t);
  const name = writeSegment(dir, 'web-01#a', 0, 0, ['{"n":1}', '{"n":2}', '{"n":3}']);
  const stream = listStreams(dir)[0];
  const cursor = cursorAt(name);

  const batch = readBatch({ spoolDir: dir, stream, cursor, limit: 10 });
  assert.equal(batch.count, 3);
  assert.deepEqual(batch.records, ['{"n":1}', '{"n":2}', '{"n":3}']);
  assert.equal(batch.firstSeq, 0);
  assert.equal(batch.position.offset, Buffer.byteLength('{"n":1}\n{"n":2}\n{"n":3}\n'));
});

test('readBatch honours the batch limit', (t) => {
  const dir = tempSpool(t);
  const name = writeSegment(dir, 'web-01#a', 0, 0, ['{"n":1}', '{"n":2}', '{"n":3}']);
  const stream = listStreams(dir)[0];
  const cursor = cursorAt(name);

  const first = readBatch({ spoolDir: dir, stream, cursor, limit: 2 });
  assert.equal(first.count, 2);

  const second = readBatch({ spoolDir: dir, stream, cursor: first.position, limit: 2 });
  assert.equal(second.count, 1);
  assert.equal(second.firstSeq, 2);
});

test('readBatch returns null when the cursor is at the end', (t) => {
  const dir = tempSpool(t);
  const name = writeSegment(dir, 'web-01#a', 0, 0, ['{"n":1}']);
  const stream = listStreams(dir)[0];
  const size = fs.statSync(path.join(dir, name)).size;
  const cursor = cursorAt(name, size, 1);

  const batch = readBatch({ spoolDir: dir, stream, cursor, limit: 10 });
  assert.equal(batch, null);
});
