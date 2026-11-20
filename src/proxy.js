import http from 'node:http';
import https from 'node:https';
import { URL } from 'node:url';

/**
 * Upstream request construction.
 *
 * `forward` copies the inbound request onto a request aimed at the route's
 * upstream, applies the gateway-owned identity headers, and streams the
 * response back to the caller.
 */

/**
 * Headers the caller must never be able to inject into an upstream:
 * legacy identity headers (replaced by gateway-owned ones where the route
 * authenticates) and credentials. See docs/route-auth.md section 6.
 */
const STRIPPED_HEADERS = new Set([
  'x-forwarded-user',
  'x-auth-user',
  'x-auth-scopes',
  'x-auth-token-id',
  'x-authenticated-user',
  'x-remote-user',
  'authorization',
  'cookie',
]);

/**
 * Build the outbound header set.
 *
 * @param {object} inbound  raw inbound headers (node's `req.headers`)
 * @param {object} identity { user, scopes, tokenId } or null for public routes
 * @param {object} extra    additional headers the gateway owns (e.g. x-request-id)
 */
export function buildHeaders(inbound, identity, extra = {}) {
  const headers = {};
  for (const [name, value] of Object.entries(inbound || {})) {
    if (!STRIPPED_HEADERS.has(name.toLowerCase())) headers[name] = value;
  }

  if (identity && identity.user) {
    headers['x-auth-user'] = identity.user;
    headers['x-auth-scopes'] = identity.scopes || '';
  }
  if (identity && identity.tokenId) {
    headers['x-auth-token-id'] = identity.tokenId;
  }
  for (const [name, value] of Object.entries(extra)) {
    headers[name] = value;
  }
  return headers;
}

/** Join the route's upstream with the inbound request target. */
export function upstreamTarget(upstream, url) {
  const base = new URL(upstream);
  return new URL(url || '/', base);
}

function transportFor(target) {
  return target.protocol === 'https:' ? https : http;
}

/**
 * Forward `req` to `upstream` and pipe the upstream response back to `res`.
 *
 * @param {object} ctx
 * @param {http.IncomingMessage} ctx.req
 * @param {http.ServerResponse} ctx.res
 * @param {string} ctx.upstream
 * @param {object|null} ctx.identity
 * @param {string} [ctx.url]  request target to send upstream (defaults to req.url)
 * @param {number} [ctx.timeoutMs]
 * @param {object} [ctx.extraHeaders]
 */
export function forward({ req, res, upstream, identity, url, timeoutMs = 15000, extraHeaders = {} }) {
  const target = upstreamTarget(upstream, url ?? req.url);
  const headers = buildHeaders(req.headers, identity, {
    host: target.host,
    ...extraHeaders,
  });

  const options = {
    protocol: target.protocol,
    hostname: target.hostname,
    port: target.port || (target.protocol === 'https:' ? 443 : 80),
    method: req.method,
    path: `${target.pathname}${target.search}`,
    headers,
    timeout: timeoutMs,
  };

  let settled = false;
  const fail = (status, code) => {
    if (settled) return;
    settled = true;
    if (!res.headersSent) {
      res.writeHead(status, { 'content-type': 'application/json' });
    }
    res.end(JSON.stringify({ error: code }));
  };

  const outbound = transportFor(target).request(options, (upstreamRes) => {
    if (settled) {
      upstreamRes.resume();
      return;
    }
    settled = true;
    res.writeHead(upstreamRes.statusCode || 502, upstreamRes.headers);
    upstreamRes.pipe(res);
  });

  outbound.on('timeout', () => {
    outbound.destroy();
    fail(504, 'upstream-timeout');
  });
  outbound.on('error', () => {
    fail(502, 'upstream-unavailable');
  });

  req.pipe(outbound);
}
