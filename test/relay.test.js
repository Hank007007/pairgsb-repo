'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { Relay } = require('../src/daemon');
const { startCollector } = require('./support/collector');

function tempDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'sealferry-relay-'));
}

function writeSource(file, events) {
  fs.writeFileSync(file, events.map((e) => `${JSON.stringify(e)}\n`).join(''), 'utf8');
}

test('a single batch travels from source file to collector', async () => {
  const dir = tempDir();
  const sourceFile = path.join(dir, 'events.log');
  writeSource(sourceFile, [
    { seq: 1, ts: 1700000000000, prev_hash: null, payload: { message: 'hello' }, meta: { host: 'a' } },
    { seq: 2, ts: 1700000001000, prev_hash: 'h1', payload: { message: 'world' }, meta: { host: 'a' } },
  ]);

  const collector = await startCollector();

  const relay = new Relay({
    source: 'node-a',
    sourcePath: sourceFile,
    spoolDir: path.join(dir, 'spool'),
    collectorUrl: collector.url,
    deliveryOptions: { attempts: 1, backoffMs: 0, timeoutMs: 2000 },
  });

  relay.start();
  const ingested = relay.ingest();
  assert.equal(ingested.ingested, 2);

  const result = await relay.flush();
  await collector.close();

  assert.equal(result.ok, true);
  assert.equal(result.sent, 2);
  assert.equal(collector.received.length, 1);

  const body = collector.received[0];
  assert.equal(body.source, 'node-a');
  assert.equal(body.prevDigest, null);
  assert.deepEqual(
    body.records.map((r) => r.seq),
    [1, 2],
  );
  assert.equal(body.records[0].payload.message, 'hello');
  assert.equal(body.digest.length, 64);
});

test('an acknowledged batch clears the spool and advances the cursor', async () => {
  const dir = tempDir();
  const sourceFile = path.join(dir, 'events.log');
  writeSource(sourceFile, [
    { seq: 1, ts: 1700000000000, prev_hash: null, payload: { message: 'one' } },
  ]);

  const collector = await startCollector();
  const spoolDir = path.join(dir, 'spool');

  const relay = new Relay({
    source: 'node-a',
    sourcePath: sourceFile,
    spoolDir,
    collectorUrl: collector.url,
    deliveryOptions: { attempts: 1, backoffMs: 0, timeoutMs: 2000 },
  });

  relay.start();
  relay.ingest();
  await relay.flush();
  await collector.close();

  assert.equal(relay.offset, fs.statSync(sourceFile).size);

  const restarted = new Relay({
    source: 'node-a',
    sourcePath: sourceFile,
    spoolDir,
    collectorUrl: collector.url,
    deliveryOptions: { attempts: 1, backoffMs: 0, timeoutMs: 2000 },
  });
  restarted.start();

  assert.equal(restarted.offset, fs.statSync(sourceFile).size);
  assert.deepEqual(restarted.spool.pending(), []);
});
