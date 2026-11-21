'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const { createCollector } = require('../src/collector/server');
const { Store } = require('../src/collector/store');
const { buildManifest, digestPayload, serializeManifest, signManifest } = require('../src/protocol/manifest');

const SECRET = 'collector-secret';
const ORIGIN = 'web-01#a';
const KEYRING = { 'edge-a': { secret: SECRET, origins: [ORIGIN] } };

function tempDir(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'spoolbatch-collector-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

async function startCollector(t, storeDir) {
  const server = createCollector({ storeDir, keyring: KEYRING });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise((resolve) => server.close(resolve)));
  return { server, port: server.address().port };
}

function batchRequest(port, { origin = ORIGIN, epoch = 0, firstSeq, records }) {
  const body = records.map((r) => `${r}\n`).join('');
  const manifest = signManifest(
    buildManifest({
      origin,
      epoch,
      firstSeq,
      count: records.length,
      digest: digestPayload(body),
      keyId: 'edge-a',
    }),
    SECRET,
  );
  return new Promise((resolve, reject) => {
    const payload = Buffer.from(body, 'utf8');
    const req = http.request(
      {
        host: '127.0.0.1',
        port,
        path: '/batches',
        method: 'POST',
        headers: {
          'content-type': 'application/x-spoolbatch',
          'content-length': String(payload.length),
          'x-spoolbatch-manifest': Buffer.from(serializeManifest(manifest), 'utf8').toString('base64'),
        },
      },
      (res) => {
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => {
          const text = Buffer.concat(chunks).toString('utf8');
          resolve({ status: res.statusCode, json: text ? JSON.parse(text) : null });
        });
      },
    );
    req.on('error', reject);
    req.end(payload);
  });
}

function getStatus(port) {
  return new Promise((resolve, reject) => {
    const req = http.request(
      { host: '127.0.0.1', port, path: '/status', method: 'GET' },
      (res) => {
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => resolve({ status: res.statusCode, json: JSON.parse(Buffer.concat(chunks).toString('utf8')) }));
      },
    );
    req.on('error', reject);
    req.end();
  });
}

test('collector accepts an in-order batch and advances the watermark', async (t) => {
  const storeDir = tempDir(t);
  const { port } = await startCollector(t, storeDir);

  const res = await batchRequest(port, {
    firstSeq: 0,
    records: ['{"n":0}', '{"n":1}', '{"n":2}'],
  });
  assert.equal(res.status, 200);
  assert.equal(res.json.accepted, 3);
  assert.equal(res.json.watermark, 2);

  const store = new Store(storeDir);
  const stored = store.recordsFor(ORIGIN, 0);
  assert.equal(stored.length, 3);
  assert.deepEqual(stored.map((r) => r.seq), [0, 1, 2]);

  const status = await getStatus(port);
  assert.equal(status.json.streams.length, 1);
  assert.equal(status.json.streams[0].watermark, 2);
  assert.equal(status.json.streams[0].hasGap, false);

  const next = await batchRequest(port, {
    firstSeq: 3,
    records: ['{"n":3}', '{"n":4}'],
  });
  assert.equal(next.status, 200);
  assert.equal(next.json.accepted, 2);
  assert.equal(next.json.watermark, 4);
  assert.equal(store.recordsFor(ORIGIN, 0).length, 5);
});
