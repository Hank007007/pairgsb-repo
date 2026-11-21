'use strict';

const http = require('node:http');

const { verifyManifest, verifyPayload, VerifyError } = require('./verify');
const { DedupTable } = require('./dedup');
const { GapTracker } = require('./gap');
const { Store } = require('./store');

const MANIFEST_HEADER = 'x-spoolbatch-manifest';

function readBody(req, limitBytes = 8 * 1024 * 1024) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let size = 0;
    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > limitBytes) {
        reject(new VerifyError('payload too large', { code: 'payload_too_large', field: 'payload' }));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', reject);
  });
}

function sendJson(res, status, value) {
  const body = Buffer.from(`${JSON.stringify(value)}\n`, 'utf8');
  res.writeHead(status, {
    'content-type': 'application/json',
    'content-length': String(body.length),
  });
  res.end(body);
}

function createCollector(options) {
  const keyring = options.keyring || {};
  const store = options.store instanceof Store ? options.store : new Store(options.storeDir);
  const dedup = options.dedup || new DedupTable(options);
  const gaps = options.gaps || new GapTracker();

  async function handleBatch(req, res) {
    const header = req.headers[MANIFEST_HEADER];
    if (!header) {
      sendJson(res, 400, {
        error: { code: 'manifest_missing', field: 'manifest', message: 'missing manifest header' },
      });
      return;
    }
    const rawManifest = Buffer.from(String(header), 'base64').toString('utf8');

    let manifest;
    try {
      manifest = verifyManifest(rawManifest, { keyring });
    } catch (err) {
      const status = err.code === 'unknown_key' ? 401 : 400;
      sendJson(res, status, {
        error: {
          code: err.code,
          field: err.field,
          origin: err.origin,
          keyId: err.keyId,
          message: err.message,
        },
      });
      return;
    }

    const body = await readBody(req);
    let records;
    try {
      records = verifyPayload(manifest, body);
    } catch (err) {
      sendJson(res, 400, {
        error: {
          code: err.code,
          field: err.field,
          origin: err.origin,
          keyId: err.keyId,
          message: err.message,
        },
      });
      return;
    }

    let accepted = 0;
    let duplicates = 0;
    let watermark = gaps.watermarkFor(manifest.origin);
    const receivedAt = new Date().toISOString();

    for (let index = 0; index < records.length; index += 1) {
      const seq = manifest.firstSeq + index;
      const isNew = dedup.accept({
        origin: manifest.origin,
        epoch: manifest.epoch,
        seq,
        at: Date.parse(receivedAt),
      });
      if (!isNew) {
        duplicates += 1;
        continue;
      }
      store.put({
        origin: manifest.origin,
        epoch: manifest.epoch,
        seq,
        record: records[index],
        receivedAt,
      });
      accepted += 1;
      const observed = gaps.observe(manifest.origin, seq);
      watermark = observed.watermark;
    }

    sendJson(res, 200, {
      accepted,
      duplicates,
      origin: manifest.origin,
      epoch: manifest.epoch,
      watermark,
      status: gaps.describe(manifest.origin),
    });
  }

  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://collector.local');
    if (req.method === 'POST' && url.pathname === '/batches') {
      handleBatch(req, res).catch((err) => {
        sendJson(res, 500, { error: { code: 'internal_error', message: err.message } });
      });
      return;
    }
    if (req.method === 'GET' && url.pathname === '/status') {
      sendJson(res, 200, { streams: gaps.report() });
      return;
    }
    sendJson(res, 404, { error: { code: 'not_found', message: `no route for ${req.method} ${url.pathname}` } });
  });

  server.collector = { store, dedup, gaps, keyring };
  return server;
}

module.exports = {
  MANIFEST_HEADER,
  createCollector,
};
