import http from 'node:http';
import { randomUUID } from 'node:crypto';
import { pathToFileURL } from 'node:url';

import { loadConfig } from './config.js';
import { matchRoute } from './match.js';
import { readScopes, formatScopes, hasScopes } from './scope.js';
import { createIntrospector } from './introspect.js';
import { bearerToken, verifyToken } from './jwt.js';
import { forward } from './proxy.js';

function sendJson(res, status, body, extraHeaders = {}) {
  if (res.headersSent) {
    res.end();
    return;
  }
  res.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(body),
    ...extraHeaders,
  });
  res.end(body);
}

function reject(res, status, code) {
  sendJson(res, status, JSON.stringify({ error: code }));
}

/**
 * Build the gateway request handler.
 *
 * @param {object} config parsed route table
 * @param {object} [deps]  injectable collaborators, mainly for tests
 */
export function createHandler(config, deps = {}) {
  const introspector = deps.introspector ?? createIntrospector(config.introspection);
  const now = deps.now ?? (() => Date.now());

  async function authenticate(route, req) {
    if (route.auth === 'public') {
      return { ok: true, identity: null };
    }
    const token = bearerToken(req.headers['authorization']);
    if (!token) {
      return { ok: false, status: 401, code: 'missing-credential' };
    }

    if (route.auth === 'jwt') {
      const result = verifyToken(token, { secret: config.jwt.secret, issuer: config.jwt.issuer, now: now() });
      if (!result.ok) {
        return { ok: false, status: 401, code: 'invalid-credential' };
      }
      return {
        ok: true,
        identity: {
          user: result.claims.sub,
          scopes: formatScopes(readScopes(result.claims)),
          tokenId: null,
          claims: result.claims,
        },
      };
    }

    const result = await introspector.lookup(token);
    if (!result.ok || !result.payload || result.payload.active !== true) {
      return { ok: false, status: 401, code: 'inactive-token' };
    }
    const payload = result.payload;
    return {
      ok: true,
      identity: {
        user: typeof payload.sub === 'string' ? payload.sub : '',
        scopes: formatScopes(readScopes(payload)),
        tokenId: typeof payload.token_id === 'string' ? payload.token_id : null,
        claims: payload,
      },
    };
  }

  return async function handle(req, res) {
    const requestId = randomUUID();
    res.setHeader('x-request-id', requestId);

    const method = (req.method || 'GET').toUpperCase();
    if (method === 'HEAD' || method === 'OPTIONS') {
      res.writeHead(200, { allow: 'GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS' });
      res.end();
      return;
    }

    const match = matchRoute(config.routes, req);
    if (match.status === 'no-route') {
      reject(res, 404, 'no-route');
      return;
    }
    if (match.status === 'method-not-allowed') {
      reject(res, 405, 'method-not-allowed');
      return;
    }

    const { route } = match;
    let auth;
    try {
      auth = await authenticate(route, req);
    } catch {
      reject(res, 502, 'auth-unavailable');
      return;
    }
    if (!auth.ok) {
      reject(res, auth.status, auth.code);
      return;
    }

    if (auth.identity && !hasScopes(auth.identity.scopes ? auth.identity.scopes.split(' ') : [], route.scopes)) {
      reject(res, 403, 'insufficient-scope');
      return;
    }

    try {
      forward({
        req,
        res,
        upstream: route.upstream,
        identity: auth.identity,
        timeoutMs: config.server.requestTimeoutMs,
        extraHeaders: { 'x-request-id': requestId },
      });
    } catch {
      reject(res, 502, 'upstream-unavailable');
    }
  };
}

export function createServer(config, deps = {}) {
  const handler = createHandler(config, deps);
  const server = http.createServer((req, res) => {
    handler(req, res).catch(() => {
      if (!res.headersSent) res.writeHead(500, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ error: 'internal' }));
    });
  });
  server.requestTimeout = config.server.requestTimeoutMs;
  server.headersTimeout = config.server.headersTimeoutMs;
  return server;
}

export function start(config = loadConfig()) {
  const server = createServer(config);
  server.listen(config.server.port, config.server.host, () => {
    const { address, port } = server.address();
    process.stdout.write(`routeclaim listening on http://${address}:${port}\n`);
  });
  return server;
}

const entry = process.argv[1];
if (entry && import.meta.url === pathToFileURL(entry).href) {
  start();
}
