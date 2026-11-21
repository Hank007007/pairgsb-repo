'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const { shipOnce } = require('../src/forwarder/client');
const { cursorPathFor, writeCursor } = require('../src/forwarder/cursor');
const { segmentFileName } = require('../src/forwarder/spool');
const { Store } = require('../src/collector/store');
const { createCollector } = require('../src/collector/server');

const SECRET = 'edge-secret';
const ORIGIN = 'web-01#a';

function workspace(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'spoolbatch-ship-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const spoolDir = path.join(root, 'spool');
  const stateDir = path.join(root, 'state');
  const storeDir = path.join(root, 'store');
  fs.mkdirSync(spoolDir, { recursive: true });
  fs.mkdirSync(stateDir, { recursive: true });
  fs.mkdirSync(storeDir, { recursive: true });
  return { root, spoolDir, stateDir, storeDir };
}

function appendSegment(spoolDir, origin, epoch, index, records) {
  const name = segmentFileName(origin, epoch, index);
  fs.writeFileSync(path.join(spoolDir, name), records.map((r) => `${r}\n`).join(''), 'utf8');
  return name;
}

async function startCollector(t, storeDir) {
  const server = createCollector({
    storeDir,
    keyring: { 'edge-a': { secret: SECRET, origins: [ORIGIN] } },
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise((resolve) => server.close(resolve)));
  return `http://127.0.0.1:${server.address().port}`;
}

function seedCursor(stateDir, origin, epoch, segment, offset, nextSeq) {
  const cursorPath = cursorPathFor(stateDir, origin, epoch);
  writeCursor(cursorPath, {
    version: 1,
    origin,
    epoch,
    segment,
    offset,
    nextSeq,
    ackedSeq: nextSeq - 1,
    updatedAt: new Date().toISOString(),
  });
  return cursorPath;
}

test('agent ships a batch in order and the collector stores it once', async (t) => {
  const { spoolDir, stateDir, storeDir } = workspace(t);
  const collectorUrl = await startCollector(t, storeDir);

  const first = appendSegment(spoolDir, ORIGIN, 0, 0, ['{"n":0}', '{"n":1}']);
  const second = appendSegment(spoolDir, ORIGIN, 0, 1, ['{"n":2}', '{"n":3}', '{"n":4}']);

  const cursorPath = path.join(stateDir, 'cursors', `${encodeURIComponent(ORIGIN)}.0.cursor.json`);
  fs.mkdirSync(path.dirname(cursorPath), { recursive: true });
  seedCursor(stateDir, ORIGIN, 0, first, 0, 0);

  const results = await shipOnce({
    spoolDir,
    stateDir,
    collectorUrl,
    keyId: 'edge-a',
    secret: SECRET,
    limit: 10,
  });

  assert.equal(results.length, 1);
  assert.equal(results[0].origin, ORIGIN);
  assert.equal(results[0].count, 5);
  assert.equal(results[0].response.accepted, 5);
  assert.equal(results[0].response.watermark, 4);

  const store = new Store(storeDir);
  assert.deepEqual(store.recordsFor(ORIGIN, 0).map((r) => r.seq), [0, 1, 2, 3, 4]);

  const cursor = JSON.parse(fs.readFileSync(cursorPath, 'utf8'));
  assert.equal(cursor.segment, second);
  assert.equal(cursor.nextSeq, 5);

  const again = await shipOnce({
    spoolDir,
    stateDir,
    collectorUrl,
    keyId: 'edge-a',
    secret: SECRET,
    limit: 10,
  });
  assert.deepEqual(again, []);
  assert.equal(store.recordsFor(ORIGIN, 0).length, 5);
});
