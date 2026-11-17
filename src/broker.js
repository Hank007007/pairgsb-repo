import http from 'node:http';
import https from 'node:https';

const DEFAULT_TIMEOUT_MS = 10_000;

function normalizeBundle(payload) {
  if (payload === null || typeof payload !== 'object') {
    throw new Error('broker returned an unexpected payload');
  }

  const { leaseId, ttlSeconds, secrets } = payload;

  if (typeof leaseId !== 'string' || leaseId.length === 0) {
    throw new Error('broker response is missing a lease id');
  }

  if (secrets === null || typeof secrets !== 'object' || Array.isArray(secrets)) {
    throw new Error('broker response is missing a secret set');
  }

  const normalized = {};
  for (const [name, value] of Object.entries(secrets)) {
    if (typeof value !== 'string') {
      throw new Error(`secret ${JSON.stringify(name)} is not a string`);
    }
    normalized[name] = value;
  }

  return {
    leaseId,
    ttlSeconds: Number.isFinite(ttlSeconds) ? ttlSeconds : undefined,
    secrets: normalized,
  };
}

function readBody(response) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    response.on('data', (chunk) => chunks.push(chunk));
    response.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    response.on('error', reject);
  });
}

export class BrokerClient {
  constructor({ brokerUrl, service, token, timeoutMs = DEFAULT_TIMEOUT_MS }) {
    this.brokerUrl = brokerUrl.replace(/\/+$/, '');
    this.service = service;
    this.token = token;
    this.timeoutMs = timeoutMs;
  }

  fetchBundle() {
    return this.#request('GET', `/v1/secrets/${encodeURIComponent(this.service)}`);
  }

  renewLease(leaseId) {
    return this.#request('POST', `/v1/leases/${encodeURIComponent(leaseId)}/renew`);
  }

  #request(method, pathname) {
    const url = new URL(`${this.brokerUrl}${pathname}`);
    const transport = url.protocol === 'https:' ? https : http;

    const headers = { accept: 'application/json' };
    if (this.token) {
      headers.authorization = `Bearer ${this.token}`;
    }

    return new Promise((resolve, reject) => {
      const request = transport.request(
        url,
        { method, headers, timeout: this.timeoutMs },
        async (response) => {
          let body;
          try {
            body = await readBody(response);
          } catch (err) {
            reject(err);
            return;
          }

          if (response.statusCode < 200 || response.statusCode >= 300) {
            reject(new Error(`broker responded with status ${response.statusCode}`));
            return;
          }

          let payload;
          try {
            payload = JSON.parse(body);
          } catch {
            reject(new Error('broker returned invalid JSON'));
            return;
          }

          try {
            resolve(normalizeBundle(payload));
          } catch (err) {
            reject(err);
          }
        },
      );

      request.on('timeout', () => {
        request.destroy(new Error('broker request timed out'));
      });
      request.on('error', reject);
      request.end();
    });
  }
}
