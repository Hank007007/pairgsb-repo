'use strict';

const { canonicalHost } = require('./routing');

const REDIRECT_STATUSES = new Set([301, 302, 303, 307, 308]);
const BODY_PRESERVING_STATUSES = new Set([307, 308]);

class RedirectError extends Error {
  constructor(message, code) {
    super(message);
    this.name = 'RedirectError';
    this.code = code || 'EBADREDIRECT';
  }
}

function isRedirect(status) {
  return REDIRECT_STATUSES.has(status);
}

function preservesBody(status) {
  return BODY_PRESERVING_STATUSES.has(status);
}

/**
 * Resolves a Location header value against the URL of the response it came
 * from. Only http and https targets are accepted.
 */
function resolveLocation(baseUrl, location) {
  if (typeof location !== 'string') {
    throw new RedirectError('redirect without a usable Location header');
  }
  let next;
  try {
    next = new URL(location, baseUrl);
  } catch (err) {
    throw new RedirectError(`cannot resolve Location ${JSON.stringify(location)}`);
  }
  if (next.protocol !== 'http:' && next.protocol !== 'https:') {
    throw new RedirectError(`unsupported redirect scheme: ${next.protocol}`);
  }
  if (next.username !== '' || next.password !== '') {
    throw new RedirectError('redirect Location must not carry credentials');
  }
  return next;
}

function copyHeaders(headers) {
  const out = new Map();
  for (const [lower, entry] of headers) out.set(lower, { name: entry.name, value: entry.value });
  return out;
}

/**
 * Builds the request for the hop after a redirect.
 *
 * `previous` describes the hop that produced the response:
 *   { url, method, headers, body }
 * where `headers` is a Map keyed by lower-case name, each value
 * `{ name, value }`.
 */
function follow(previous, response) {
  const status = response.statusCode;
  if (!isRedirect(status)) {
    throw new RedirectError(`status ${status} is not a redirect`);
  }

  const next = resolveLocation(previous.url, response.headers.location);
  const headers = copyHeaders(previous.headers);
  headers.delete('host');
  headers.delete('content-length');

  const method = preservesBody(status) ? previous.method : status === 303 ? 'GET' : redirectMethod(previous.method);
  let body = preservesBody(status) ? previous.body : null;

  if (method === 'GET' || method === 'HEAD') {
    body = null;
    headers.delete('content-type');
  }

  return {
    url: next,
    host: canonicalHost(next.hostname),
    port: next.port === '' ? (next.protocol === 'https:' ? 443 : 80) : Number(next.port),
    protocol: next.protocol,
    path: `${next.pathname}${next.search}`,
    method,
    headers,
    body,
    hop: (previous.hop || 0) + 1,
  };
}

function redirectMethod(previousMethod) {
  const upper = String(previousMethod || 'GET').toUpperCase();
  if (upper === 'GET' || upper === 'HEAD') return upper;
  return 'GET';
}

module.exports = {
  RedirectError,
  isRedirect,
  preservesBody,
  resolveLocation,
  follow,
};
