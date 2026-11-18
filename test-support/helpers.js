import http from 'node:http';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { importJWK, SignJWT } from 'jose';
import { keySetFromJwks } from '../src/jwt.js';
import { normalizeConfig } from '../src/config.js';
import { createProxy } from '../src/proxy.js';

const here = path.dirname(fileURLToPath(import.meta.url));
export const projectRoot = path.resolve(here, '..');
export const keysDir = path.join(projectRoot, 'keys');

export const ISSUER = 'https://auth.internal.hopguard';
export const AUDIENCE = 'hopguard-edge';

let cachedPrivateKey = null;

async function privateKey() {
  if (cachedPrivateKey === null) {
    const jwk = JSON.parse(await readFile(path.join(keysDir, 'test-private.jwk.json'), 'utf8'));
    cachedPrivateKey = await importJWK({ ...jwk, alg: 'RS256' }, 'RS256');
  }
  return cachedPrivateKey;
}

export async function loadTestKeySet() {
  const jwks = JSON.parse(await readFile(path.join(keysDir, 'jwks.json'), 'utf8'));
  return keySetFromJwks(jwks);
}

/**
 * Signs a session token. Claim defaults describe a healthy token; individual
 * claims can be overridden or removed with `undefined`.
 */
export async function signToken(claims = {}, { alg = 'RS256', kid = 'local-test-1' } = {}) {
  const now = Math.floor(Date.now() / 1000);
  const payload = {
    sub: 'user-123',
    roles: ['operator'],
    scope: ['orders:read'],
    iss: ISSUER,
    aud: AUDIENCE,
    iat: now,
    nbf: now - 5,
    exp: now + 600,
    ...claims,
  };
  for (const key of Object.keys(payload)) {
    if (payload[key] === undefined) {
      delete payload[key];
    }
  }

  const header = { alg };
  if (kid !== undefined) {
    header.kid = kid;
  }
  return new SignJWT(payload).setProtectedHeader(header).sign(await privateKey());
}

export function rawToken(payload, { alg = 'RS256', kid = 'local-test-1' } = {}) {
  const encode = (value) =>
    Buffer.from(JSON.stringify(value), 'utf8').toString('base64url');
  const header = { alg };
  if (kid !== undefined) {
    header.kid = kid;
  }
  return `${encode(header)}.${encode(payload)}.c2lnbmF0dXJl`;
}

/** Starts an upstream that echoes the request it received. */
export async function startEchoUpstream() {
  const requests = [];
  const server = http.createServer((request, response) => {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk) => {
      body += chunk;
    });
    request.on('end', () => {
      requests.push({
        method: request.method,
        url: request.url,
        headers: { ...request.headers },
        body,
      });
      const payload = JSON.stringify({
        ok: true,
        method: request.method,
        url: request.url,
        headers: request.headers,
      });
      response.writeHead(200, {
        'content-type': 'application/json',
        'content-length': Buffer.byteLength(payload),
      });
      response.end(payload);
    });
  });

  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const baseUrl = `http://127.0.0.1:${server.address().port}`;
  return { server, requests, baseUrl };
}

export function testConfig(upstreamMap) {
  return normalizeConfig({
    listen: { host: '127.0.0.1', port: 0 },
    token: {
      issuer: ISSUER,
      audience: AUDIENCE,
      algorithms: ['RS256'],
      clockToleranceSeconds: 30,
    },
    defaultRoute: 'public-site',
    upstreams: upstreamMap,
    routes: [
      {
        id: 'admin-reports',
        prefix: '/admin',
        upstream: 'reports',
        auth: { required: true, scopes: ['admin:export'] },
      },
      {
        id: 'orders',
        prefix: '/orders',
        upstream: 'orders',
        auth: { required: true, scopes: ['orders:read'] },
      },
      {
        id: 'catalog',
        prefix: '/catalog',
        upstream: 'catalog',
        auth: { required: true, scopes: ['catalog:read'] },
      },
      {
        id: 'public-site',
        prefix: '/public',
        upstream: 'public-site',
        auth: { required: false },
      },
    ],
  });
}

/**
 * Starts the gateway in front of a single echo upstream registered under every
 * upstream name used by the test route table.
 */
export async function startGateway() {
  const upstream = await startEchoUpstream();
  const upstreamMap = {
    'public-site': upstream.baseUrl,
    orders: upstream.baseUrl,
    catalog: upstream.baseUrl,
    reports: upstream.baseUrl,
  };
  const config = testConfig(upstreamMap);
  const keys = await loadTestKeySet();

  const server = createProxy({
    routes: config.routes,
    upstreams: config.upstreams,
    defaultRoute: config.defaultRoute,
    keys,
    token: config.token,
    requestTimeoutMs: 5_000,
  });

  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = `http://127.0.0.1:${server.address().port}`;

  return {
    baseUrl: address,
    requests: upstream.requests,
    upstream,
    async close() {
      await new Promise((resolve) => server.close(resolve));
      await new Promise((resolve) => upstream.server.close(resolve));
    },
  };
}

/** Sends a request and buffers the JSON body. */
export async function call(baseUrl, requestPath, { method = 'GET', headers = {}, body } = {}) {
  const response = await fetch(`${baseUrl}${requestPath}`, { method, headers, body });
  const text = await response.text();
  let json = null;
  try {
    json = JSON.parse(text);
  } catch {
    json = null;
  }
  return { status: response.status, headers: response.headers, text, json };
}
