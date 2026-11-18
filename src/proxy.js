import http from 'node:http';
import { verifyToken, TokenError } from './jwt.js';
import { matchRoute } from './routes.js';
import { missingScopes, parseScopes } from './scope.js';

const HOP_BY_HOP_HEADERS = new Set([
  'connection',
  'keep-alive',
  'proxy-authenticate',
  'proxy-authorization',
  'te',
  'trailer',
  'trailers',
  'transfer-encoding',
  'upgrade',
]);

/**
 * Builds the header block sent to the upstream service from the headers the
 * caller sent plus the identity the gateway authenticated.
 *
 * Hop-by-hop headers are never forwarded.
 */
export function forwardHeaders(clientHeaders, identity) {
  const output = {};

  if (identity !== null && identity !== undefined) {
    output['x-auth-sub'] = identity.sub;
    output['x-auth-roles'] = identity.roles.join(',');
  }

  for (const [name, value] of Object.entries(clientHeaders)) {
    const lower = name.toLowerCase();
    if (lower === 'x-auth-sub') {
      continue;
    }
    if (HOP_BY_HOP_HEADERS.has(lower)) {
      continue;
    }
    output[lower] = value;
  }

  return output;
}

function sendJson(response, statusCode, body, extraHeaders = {}) {
  const payload = JSON.stringify(body);
  response.writeHead(statusCode, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(payload),
    'cache-control': 'no-store',
    ...extraHeaders,
  });
  response.end(payload);
}

function bearerToken(request) {
  const header = request.headers.authorization;
  if (typeof header !== 'string') {
    return null;
  }
  const match = /^Bearer[ ]+(\S+)$/i.exec(header);
  return match === null ? null : match[1];
}

function authorize(route, identity) {
  const granted = parseScopes(identity.payload);
  const missing = missingScopes(granted, route.auth.scopes);
  if (missing.length > 0) {
    return { ok: false, status: 403, code: 'insufficient_scope', missing };
  }
  return { ok: true };
}

function proxyRequest(request, response, route, options, identity) {
  const upstreamBase = options.upstreams.get(route.upstream);
  if (typeof upstreamBase !== 'string' || upstreamBase.length === 0) {
    sendJson(response, 502, { error: 'bad_gateway', message: 'upstream is not configured' });
    return;
  }

  const rawTarget = typeof request.url === 'string' ? request.url : '/';
  const queryIndex = rawTarget.indexOf('?');
  const rawPath = queryIndex === -1 ? rawTarget : rawTarget.slice(0, queryIndex);
  const search = queryIndex === -1 ? '' : rawTarget.slice(queryIndex);

  if (!rawPath.startsWith('/')) {
    sendJson(response, 400, { error: 'bad_request', code: 'bad_request' });
    return;
  }

  let target;
  try {
    const parsed = new URL(`http://gateway.invalid${rawPath}`);
    target = new URL(upstreamBase);
    target.pathname = parsed.pathname;
    target.search = search;
  } catch {
    sendJson(response, 400, { error: 'bad_request', code: 'bad_request' });
    return;
  }

  const headers = forwardHeaders(request.headers, identity);
  headers['host'] = target.host;

  const upstreamRequest = http.request(
    {
      protocol: target.protocol,
      hostname: target.hostname,
      port: target.port,
      method: request.method,
      path: `${target.pathname}${target.search}`,
      headers,
      timeout: options.requestTimeoutMs,
    },
    (upstreamResponse) => {
      const responseHeaders = {};
      for (const [name, value] of Object.entries(upstreamResponse.headers)) {
        if (HOP_BY_HOP_HEADERS.has(name.toLowerCase())) {
          continue;
        }
        responseHeaders[name] = value;
      }
      response.writeHead(upstreamResponse.statusCode ?? 502, responseHeaders);
      upstreamResponse.pipe(response);
    },
  );

  upstreamRequest.on('timeout', () => {
    upstreamRequest.destroy(new Error('upstream timed out'));
  });

  upstreamRequest.on('error', (error) => {
    if (response.headersSent) {
      response.destroy();
      return;
    }
    const timedOut = error.message === 'upstream timed out';
    sendJson(response, timedOut ? 504 : 502, {
      error: timedOut ? 'gateway_timeout' : 'bad_gateway',
      message: timedOut ? 'upstream did not respond in time' : 'upstream is unavailable',
    });
  });

  request.pipe(upstreamRequest);
}

async function handleRequest(request, response, settings) {
  const route = matchRoute(request.url, settings.routes, settings.defaultRoute);

  if (route === null) {
    sendJson(response, 404, { error: 'not_found' });
    return;
  }

  let identity = null;

  if (route.auth.required) {
    const tokenValue = bearerToken(request);
    if (tokenValue === null) {
      sendJson(
        response,
        401,
        { error: 'unauthorized', code: 'missing_token' },
        { 'www-authenticate': 'Bearer realm="hopguard"' },
      );
      return;
    }

    try {
      identity = await verifyToken(tokenValue, {
        keys: settings.keys,
        token: settings.token,
      });
    } catch (error) {
      if (error instanceof TokenError) {
        sendJson(
          response,
          401,
          { error: 'unauthorized', code: error.reason },
          { 'www-authenticate': 'Bearer realm="hopguard", error="invalid_token"' },
        );
        return;
      }
      throw error;
    }

    const decision = authorize(route, identity);
    if (!decision.ok) {
      sendJson(response, 403, {
        error: 'forbidden',
        code: decision.code,
        missing_scopes: decision.missing,
      });
      return;
    }
  }

  proxyRequest(request, response, route, settings, identity);
}

/**
 * Creates the edge gateway HTTP server.
 *
 * Options:
 *   routes            route table produced by the configuration loader
 *   upstreams         Map of upstream name to base URL
 *   defaultRoute      route used when no prefix matches, or null
 *   keys              key set produced by `loadKeySet`
 *   token             token policy from the gateway configuration
 *   requestTimeoutMs  upstream timeout in milliseconds
 *   logger            optional `{ info, warn, error }` sink
 */
export function createProxy(options) {
  const {
    routes,
    upstreams,
    defaultRoute = null,
    keys,
    token,
    requestTimeoutMs = 10_000,
    logger = null,
  } = options;

  if (!Array.isArray(routes)) {
    throw new TypeError('createProxy requires a routes array');
  }
  if (!(upstreams instanceof Map)) {
    throw new TypeError('createProxy requires an upstreams Map');
  }
  if (keys === null || typeof keys !== 'object') {
    throw new TypeError('createProxy requires a keys object');
  }
  if (token === null || typeof token !== 'object') {
    throw new TypeError('createProxy requires a token policy');
  }

  const settings = {
    routes,
    upstreams,
    defaultRoute,
    keys,
    token,
    requestTimeoutMs,
  };

  return http.createServer((request, response) => {
    handleRequest(request, response, settings).catch((error) => {
      logger?.error?.('request handling failed', error);
      if (!response.headersSent) {
        sendJson(response, 500, { error: 'internal_error' });
      } else {
        response.destroy();
      }
    });
  });
}
