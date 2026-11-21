'use strict';

const http = require('node:http');

const { readManifest, verifyPayload, verifySignature, VerifyError } = require('./verify');
const { DedupTable } = require('./dedup');
const { GapTracker } = require('./gap');
const { Store } = require('./store');
const { streamKey } = require('../protocol/manifest');

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

function sendVerifyError(res, status, err) {
  sendJson(res, status, {
    error: {
      code: err.code,
      field: err.field,
      origin: err.origin,
      keyId: err.keyId,
      message: err.message,
    },
  });
}

/**
 * Returns the seq whose stored record disagrees with the payload, or null.
 * A batch that re-claims an already-stored sequence number with different
 * content disagrees with what the receiver has stored.
 */
function findStoredConflict(store, manifest, records) {
  const stored = store.recordsFor(manifest.origin, manifest.epoch);
  if (stored.length === 0) return null;
  const bySeq = new Map(stored.map((entry) => [entry.seq, entry.record]));
  for (let index = 0; index < records.length; index += 1) {
    const seq = manifest.firstSeq + index;
    if (bySeq.has(seq) && bySeq.get(seq) !== records[index]) return seq;
  }
  return null;
}

/**
 * Rebuilds deduplication and gap state from what is on disk, so a replay of
 * a batch accepted before a restart is still recognised as a duplicate and
 * the reported watermark does not regress.
 */
function rebuildState(store, dedup, gaps) {
  const streams = new Map();
  for (const entry of store.entries()) {
    dedup.accept({ origin: entry.origin, epoch: entry.epoch, seq: entry.seq });
    const key = streamKey(entry.origin, entry.epoch);
    let group = streams.get(key);
    if (!group) {
      group = { origin: entry.origin, epoch: entry.epoch, seqs: [] };
      streams.set(key, group);
    }
    group.seqs.push(entry.seq);
  }
  for (const group of streams.values()) {
    group.seqs.sort((a, b) => a - b);
    for (const seq of group.seqs) {
      gaps.observe(group.origin, group.epoch, seq);
    }
  }
}

function createCollector(options) {
  const keyring = options.keyring || {};
  const store = options.store instanceof Store ? options.store : new Store(options.storeDir);
  const dedup = options.dedup || new DedupTable(options);
  const gaps = options.gaps || new GapTracker();

  rebuildState(store, dedup, gaps);

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
      manifest = readManifest(rawManifest);
    } catch (err) {
      sendVerifyError(res, 400, err);
      return;
    }

    const body = await readBody(req);
    let records;
    try {
      records = verifyPayload(manifest, body);
    } catch (err) {
      sendVerifyError(res, 400, err);
      return;
    }

    const conflict = findStoredConflict(store, manifest, records);
    if (conflict !== null) {
      sendJson(res, 409, {
        error: {
          code: 'record_conflict',
          field: 'firstSeq',
          origin: manifest.origin,
          keyId: manifest.keyId,
          message:
            `seq ${conflict} for origin ${manifest.origin} is already stored with different content`,
        },
      });
      return;
    }

    try {
      verifySignature(manifest, keyring);
    } catch (err) {
      const status =
        err.code === 'unknown_key' ? 401 : err.code === 'origin_not_authorised' ? 403 : 400;
      sendVerifyError(res, status, err);
      return;
    }

    let accepted = 0;
    let duplicates = 0;
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
      gaps.observe(manifest.origin, manifest.epoch, seq);
    }

    sendJson(res, 200, {
      accepted,
      duplicates,
      origin: manifest.origin,
      epoch: manifest.epoch,
      watermark: gaps.watermarkFor(manifest.origin, manifest.epoch),
      status: gaps.describe(manifest.origin, manifest.epoch),
    });
  }

  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://collector.local');
    if (req.method === 'POST' && url.pathname === '/batches') {
      handleBatch(req, res).catch((err) => {
        if (err && err.code === 'payload_too_large') {
          sendJson(res, 413, {
            error: { code: err.code, field: err.field, message: err.message },
          });
          return;
        }
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
