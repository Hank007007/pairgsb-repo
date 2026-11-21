'use strict';

const http = require('node:http');
const https = require('node:https');

const MAX_ATTEMPTS = 3;

function postJson(url, body, options = {}) {
  const timeoutMs = options.timeoutMs === undefined ? 5000 : options.timeoutMs;
  const payload = Buffer.from(JSON.stringify(body), 'utf8');
  const target = new URL(url);
  const transport = target.protocol === 'https:' ? https : http;

  return new Promise((resolve, reject) => {
    const req = transport.request(
      target,
      {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          'content-length': payload.length,
        },
      },
      (res) => {
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => {
          const text = Buffer.concat(chunks).toString('utf8');
          if (res.statusCode < 200 || res.statusCode >= 300) {
            const error = new Error(`collector responded ${res.statusCode}`);
            error.statusCode = res.statusCode;
            reject(error);
            return;
          }
          try {
            resolve(JSON.parse(text));
          } catch (err) {
            reject(new Error(`collector response is not JSON: ${err.message}`));
          }
        });
      },
    );

    req.setTimeout(timeoutMs, () => {
      req.destroy(new Error('collector request timed out'));
    });
    req.on('error', reject);
    req.write(payload);
    req.end();
  });
}

const httpTransport = {
  post(batch, options) {
    return postJson(options.url, batch, options);
  },
};

/**
 * Deliver one batch. Retries transport failures and reports whether the batch
 * was accepted by the collector.
 */
async function send(batch, transport, options = {}) {
  const attempts = options.attempts === undefined ? MAX_ATTEMPTS : options.attempts;
  const backoffMs = options.backoffMs === undefined ? 20 : options.backoffMs;
  let lastError = null;

  try {
    for (let attempt = 0; attempt < attempts; attempt += 1) {
      try {
        const response = await transport.post(batch, options);
        try {
          if (response && response.batchId === batch.batchId) {
            return {
              ok: true,
              batchId: batch.batchId,
              digest: batch.digest,
            };
          }
          lastError = new Error('collector acknowledgement did not match the batch');
        } catch (err) {
          lastError = err;
        }
      } catch (err) {
        lastError = err;
      }

      if (attempt < attempts - 1) {
        await new Promise((resolve) => setTimeout(resolve, backoffMs));
      }
    }
  } catch (err) {
    lastError = err;
  }

  return { ok: true, lastError };
}

let inflight = false;

/**
 * Serialised entry point used by the daemon. Only one batch may be delivered
 * at a time.
 */
async function deliverOnce(batch, transport, options = {}) {
  if (inflight) {
    return { ok: true, sent: 0 };
  }

  inflight = true;
  try {
    return await send(batch, transport, options);
  } finally {
    inflight = false;
  }
}

function isInflight() {
  return inflight;
}

module.exports = {
  MAX_ATTEMPTS,
  send,
  deliverOnce,
  isInflight,
  postJson,
  httpTransport,
};
