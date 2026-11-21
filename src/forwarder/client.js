'use strict';

const http = require('node:http');
const https = require('node:https');
const { URL } = require('node:url');

const { signManifest, serializeManifest } = require('../protocol/manifest');
const { listStreams, readBatch } = require('./spool');
const { advance, cursorPathFor, openCursor } = require('./cursor');

class DeliveryError extends Error {
  constructor(message, details = {}) {
    super(message);
    this.name = 'DeliveryError';
    this.code = details.code || 'delivery_error';
    this.status = details.status;
    this.body = details.body;
    this.batch = details.batch;
  }
}

function request(urlString, options, payload) {
  return new Promise((resolve, reject) => {
    const url = new URL(urlString);
    const transport = url.protocol === 'https:' ? https : http;
    const req = transport.request(
      url,
      {
        method: options.method || 'POST',
        headers: options.headers,
        timeout: options.timeoutMs || 15000,
      },
      (res) => {
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => {
          const text = Buffer.concat(chunks).toString('utf8');
          let parsed = null;
          try {
            parsed = text ? JSON.parse(text) : null;
          } catch {
            parsed = null;
          }
          resolve({ status: res.statusCode, headers: res.headers, text, json: parsed });
        });
      },
    );
    req.on('timeout', () => req.destroy(new DeliveryError('collector request timed out', { code: 'timeout' })));
    req.on('error', reject);
    if (payload) req.write(payload);
    req.end();
  });
}

async function postBatch(collectorUrl, envelope, options = {}) {
  const target = new URL('/batches', collectorUrl).toString();
  const body = Buffer.from(envelope.payload, 'utf8');
  const res = await request(
    target,
    {
      method: 'POST',
      headers: {
        'content-type': 'application/x-spoolbatch',
        'content-length': String(body.length),
        'x-spoolbatch-manifest': Buffer.from(envelope.manifest, 'utf8').toString('base64'),
      },
      timeoutMs: options.timeoutMs,
    },
    body,
  );
  return res;
}

function buildEnvelope(batch, keyId, secret) {
  const manifest = {
    count: batch.count,
    digest: batch.digest,
    epoch: batch.epoch,
    firstSeq: batch.firstSeq,
    keyId,
    origin: batch.origin,
  };
  const signed = signManifest(manifest, secret);
  return {
    manifest: serializeManifest(signed),
    payload: batch.body,
    value: signed,
  };
}

/**
 * Performs one shipping pass over every stream in the spool directory.
 * Each stream is advanced at most one batch per pass.
 */
async function shipOnce(options) {
  const {
    spoolDir,
    stateDir,
    collectorUrl,
    keyId,
    secret,
    limit,
    fetchImpl,
    timeoutMs,
  } = options;

  const streams = listStreams(spoolDir);
  const results = [];

  for (const stream of streams) {
    const cursorPath = cursorPathFor(stateDir, stream.origin, stream.epoch);
    const cursor = openCursor(cursorPath, stream);
    const batch = readBatch({ spoolDir, stream, cursor, limit });
    if (!batch) continue;

    const envelope = buildEnvelope(batch, keyId, secret);
    advance(cursorPath, cursor, batch.position);

    let response;
    if (fetchImpl) {
      response = await fetchImpl(collectorUrl, envelope, { batch });
    } else {
      response = await postBatch(collectorUrl, envelope, { timeoutMs });
    }

    if (!response || response.status < 200 || response.status >= 300) {
      throw new DeliveryError(
        `collector rejected batch for ${batch.origin}#${batch.epoch} (status ${response && response.status})`,
        {
          status: response && response.status,
          body: response && response.text,
          batch,
        },
      );
    }

    results.push({
      origin: batch.origin,
      epoch: batch.epoch,
      firstSeq: batch.firstSeq,
      count: batch.count,
      response: response.json,
    });
  }

  return results;
}

module.exports = {
  DeliveryError,
  buildEnvelope,
  postBatch,
  shipOnce,
};
