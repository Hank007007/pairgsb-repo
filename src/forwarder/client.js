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
    this.cause = details.cause;
  }
}

const DEFAULT_RETRIES = 3;
const DEFAULT_RETRY_DELAY_MS = 200;

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
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
 * Sends one batch, retrying on 5xx responses and transport failures with
 * exponential backoff. A 4xx response is returned immediately: it will not
 * succeed on retry. Throws DeliveryError when transport keeps failing.
 */
async function sendBatch(collectorUrl, envelope, options = {}) {
  const retries = Number.isInteger(options.retries) ? options.retries : DEFAULT_RETRIES;
  const retryDelayMs = options.retryDelayMs ?? DEFAULT_RETRY_DELAY_MS;
  let lastResponse = null;
  let lastError = null;

  for (let attempt = 0; attempt <= retries; attempt += 1) {
    if (attempt > 0) await delay(retryDelayMs * 2 ** (attempt - 1));
    try {
      const response = options.fetchImpl
        ? await options.fetchImpl(collectorUrl, envelope, { batch: options.batch })
        : await postBatch(collectorUrl, envelope, { timeoutMs: options.timeoutMs });
      if (response && response.status >= 200 && response.status < 300) return response;
      lastResponse = response;
      lastError = null;
      if (response && response.status >= 400 && response.status < 500) break;
    } catch (err) {
      lastError = err;
    }
  }

  if (lastError) {
    if (lastError instanceof DeliveryError) throw lastError;
    throw new DeliveryError(`collector request failed: ${lastError.message}`, {
      code: 'connection_error',
      batch: options.batch,
      cause: lastError,
    });
  }
  return lastResponse;
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
    retries,
    retryDelayMs,
  } = options;

  const streams = listStreams(spoolDir);
  const results = [];

  for (const stream of streams) {
    const cursorPath = cursorPathFor(stateDir, stream.origin, stream.epoch);
    const cursor = openCursor(cursorPath, stream);
    const batch = readBatch({ spoolDir, stream, cursor, limit });
    if (!batch) continue;

    const envelope = buildEnvelope(batch, keyId, secret);
    const response = await sendBatch(collectorUrl, envelope, {
      batch,
      fetchImpl,
      timeoutMs,
      retries,
      retryDelayMs,
    });

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

    advance(cursorPath, cursor, batch.position);

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
