/**
 * Credential selection and header injection.
 */

import { splitHostPort } from './bundle.js';

export const HEADER_FOR_SCHEME = Object.freeze({
  bearer: 'Authorization',
  'api-key': 'X-Api-Key',
});

const DEFAULT_PORT = Object.freeze({
  'http:': 80,
  'https:': 443,
});

const CREDENTIAL_HEADER_NAMES = new Set(
  [
    ...Object.values(HEADER_FOR_SCHEME),
    'Proxy-Authorization',
    'Cookie',
  ].map((name) => name.toLowerCase()),
);

function stripRootDot(hostname) {
  return hostname.endsWith('.') ? hostname.slice(0, -1) : hostname;
}

/**
 * Compute the routing host for a URL: lower-case hostname plus an explicit
 * port when the URL's port is not the scheme default.
 *
 * @param {URL} url
 * @returns {string}
 */
export function effectiveHost(url) {
  const hostname = stripRootDot(url.hostname.toLowerCase());
  const defaultPort = DEFAULT_PORT[url.protocol];
  const port = url.port === '' ? null : Number(url.port);
  if (port === null || port === defaultPort) {
    return hostname;
  }
  return `${hostname}:${port}`;
}

/**
 * Whether `entry` routes to `url` under §2. An entry without a port matches
 * only the request's default port; an entry with a port matches that exact
 * port.
 *
 * @param {URL} url
 * @param {{ host: string }} entry
 * @returns {boolean}
 */
function hostMatches(url, entry) {
  const { hostname: entryHost, port: entryPort } = splitHostPort(entry.host);
  const requestHost = stripRootDot(url.hostname.toLowerCase());
  if (requestHost !== entryHost) {
    return false;
  }

  const defaultPort = DEFAULT_PORT[url.protocol];
  const requestPort = url.port === '' ? defaultPort : Number(url.port);
  if (requestPort === undefined) {
    return false;
  }
  if (entryPort === null) {
    return requestPort === defaultPort;
  }
  return requestPort === Number(entryPort);
}

function toMillis(value) {
  if (value instanceof Date) {
    return value.getTime();
  }
  if (typeof value === 'number') {
    return value;
  }
  if (typeof value === 'string') {
    return new Date(value).getTime();
  }
  return NaN;
}

/**
 * Temporal validity under §3: `not_before <= now < expires_at`, evaluating
 * each bound only when it is present.
 *
 * @param {object} entry
 * @param {number} nowMs
 * @returns {boolean}
 */
export function isTemporallyValid(entry, nowMs) {
  if (entry.notBefore !== null && entry.notBefore !== undefined) {
    if (!(nowMs >= toMillis(entry.notBefore))) {
      return false;
    }
  }
  if (entry.expiresAt !== null && entry.expiresAt !== undefined) {
    if (!(nowMs < toMillis(entry.expiresAt))) {
      return false;
    }
  }
  return true;
}

/**
 * Pick the bundle entry that supplies the credential for `url`.
 *
 * Entries are examined most-specific-first so that a rule carrying an explicit
 * port takes precedence over a portless rule.
 *
 * @param {{ entries: ReadonlyArray<object> }} bundle
 * @param {URL | string} target
 * @param {{ now?: (() => number | Date) | number | Date, clock?: (() => number | Date) | number | Date }} [options]
 * @returns {object | null}
 */
export function selectEntry(bundle, target, options = {}) {
  const url = target instanceof URL ? target : new URL(target);
  if (!bundle || !Array.isArray(bundle.entries)) {
    return null;
  }

  const clock = options.now ?? options.clock ?? (() => Date.now());
  const nowMs = toMillis(typeof clock === 'function' ? clock() : clock);
  let selected = null;
  let selectedSpecificity = -1;

  for (const entry of bundle.entries) {
    if (!hostMatches(url, entry)) {
      continue;
    }
    if (!isTemporallyValid(entry, nowMs)) {
      continue;
    }
    const specificity = splitHostPort(entry.host).port === null ? 0 : 1;
    if (specificity > selectedSpecificity) {
      selected = entry;
      selectedSpecificity = specificity;
    }
  }
  return selected;
}

/**
 * Build the credential header for an entry.
 *
 * @param {object} entry
 * @returns {{ name: string, value: string }}
 */
export function credentialHeader(entry) {
  const name = HEADER_FOR_SCHEME[entry.scheme];
  if (!name) {
    throw new Error(`unsupported scheme: ${entry.scheme}`);
  }
  const value = entry.scheme === 'bearer' ? `Bearer ${entry.value}` : entry.value;
  return { name, value };
}

/**
 * Return a copy of `headers` with any existing credential header (compared
 * case-insensitively) replaced by `entry`'s header.
 *
 * @param {Record<string, string>} headers
 * @param {object} entry
 * @returns {Record<string, string>}
 */
export function applyCredential(headers, entry) {
  const { name, value } = credentialHeader(entry);
  const result = {};
  for (const [key, existing] of Object.entries(headers ?? {})) {
    if (key.toLowerCase() === name.toLowerCase()) continue;
    result[key] = existing;
  }
  result[name] = value;
  return result;
}

/**
 * Remove relay credential headers from a header map. Used to make
 * caller-supplied credential headers hop-local across a cross-host redirect.
 *
 * @param {Record<string, string>} headers
 * @returns {Record<string, string>}
 */
export function stripCredentialHeaders(headers) {
  const result = {};
  for (const [key, value] of Object.entries(headers ?? {})) {
    if (CREDENTIAL_HEADER_NAMES.has(key.toLowerCase())) {
      continue;
    }
    result[key] = value;
  }
  return result;
}

/**
 * Select an entry for `url` and return the headers to send, plus the entry that
 * was used (or `null` when no credential applies).
 *
 * @param {{ entries: ReadonlyArray<object> }} bundle
 * @param {URL | string} target
 * @param {Record<string, string>} [headers]
 * @param {{ now?: (() => number | Date) | number | Date, clock?: (() => number | Date) | number | Date }} [options]
 * @returns {{ entry: object | null, headers: Record<string, string> }}
 */
export function injectIntoRequest(bundle, target, headers = {}, options = {}) {
  const url = target instanceof URL ? target : new URL(target);
  const entry = selectEntry(bundle, url, options);
  if (entry === null) {
    return { entry: null, headers: { ...headers } };
  }
  return { entry, headers: applyCredential(headers, entry) };
}
