/**
 * Outbound transport.
 *
 * Thin wrapper over `node:http` / `node:https`. Tests substitute a fake
 * transport with the same shape.
 */

import http from 'node:http';
import https from 'node:https';
import { TransportError, AbortedError } from './errors.js';

/**
 * @typedef {object} TransportRequest
 * @property {string} url
 * @property {string} method
 * @property {Record<string, string>} headers
 * @property {string | Buffer | null} [body]
 * @property {number} [timeoutMs]
 * @property {AbortSignal} [signal]
 *
 * @typedef {object} TransportResponse
 * @property {number} status
 * @property {Record<string, string | string[]>} headers
 * @property {string} body
 * @property {string} url
 */

/**
 * @param {TransportRequest} request
 * @returns {Promise<TransportResponse>}
 */
export function nodeTransport(request) {
  return new Promise((resolve, reject) => {
    const {
      url, method = 'GET', headers = {}, body = null, timeoutMs, signal,
    } = request;

    let target;
    try {
      target = new URL(url);
    } catch (cause) {
      reject(new TransportError(`invalid url: ${url}`, cause));
      return;
    }

    const lib = target.protocol === 'https:' ? https : (target.protocol === 'http:' ? http : null);
    if (lib === null) {
      reject(new TransportError(`unsupported protocol: ${target.protocol}`));
      return;
    }

    if (signal?.aborted) {
      reject(new AbortedError());
      return;
    }

    const req = lib.request(target, { method, headers }, (res) => {
      const chunks = [];
      res.on('data', (chunk) => chunks.push(chunk));
      res.on('end', () => {
        resolve({
          status: res.statusCode ?? 0,
          headers: res.headers,
          body: Buffer.concat(chunks).toString('utf8'),
          url: target.toString(),
        });
      });
      res.on('error', (cause) => reject(new TransportError(`response error: ${cause.message}`, cause)));
    });

    let timer = null;
    if (typeof timeoutMs === 'number' && timeoutMs > 0) {
      timer = setTimeout(() => {
        req.destroy(new TransportError(`request timed out after ${timeoutMs}ms`));
      }, timeoutMs);
      if (typeof timer.unref === 'function') timer.unref();
    }

    const onAbort = () => {
      req.destroy(new AbortedError());
    };
    signal?.addEventListener('abort', onAbort, { once: true });

    req.on('error', (cause) => {
      if (timer) clearTimeout(timer);
      signal?.removeEventListener('abort', onAbort);
      if (cause instanceof AbortedError || cause?.code === 'ABORT_ERR') {
        reject(new AbortedError());
        return;
      }
      reject(cause instanceof TransportError ? cause : new TransportError(cause.message, cause));
    });

    req.on('close', () => {
      if (timer) clearTimeout(timer);
      signal?.removeEventListener('abort', onAbort);
    });

    if (body !== null && body !== undefined) {
      req.write(body);
    }
    req.end();
  });
}
