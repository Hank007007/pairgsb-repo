import {
  canonicalKey,
  canonicalQuery,
  parseQuery,
  parseQueryPairs,
  stripProtocolPairs,
  stripProtocolParams,
} from './canonical.js';
import { candidateVersions } from './keys.js';
import {
  buildCanonicalString,
  PROTOCOL_ALG,
  PROTOCOL_EXPIRES,
  PROTOCOL_METHOD,
  PROTOCOL_TENANT,
  PROTOCOL_VERSION,
  SIG_PARAM,
} from './presign.js';
import { hmac, resolveKey, signaturesMatch } from './sign.js';

/**
 * Gateway-side verification.
 *
 * verify() takes the URL a client presented, rebuilds the canonical request,
 * and decides whether the request may proceed to the storage backend.
 */

export const CLOCK_SKEW_SECONDS = 30;
export const DEFAULT_ALG = 'HS256';

const PROTOCOL_PARAMS = new Set([
  PROTOCOL_ALG,
  PROTOCOL_METHOD,
  PROTOCOL_EXPIRES,
  PROTOCOL_TENANT,
  PROTOCOL_VERSION,
  SIG_PARAM,
]);

const ABSOLUTE_URL_PATH = /^[A-Za-z][A-Za-z0-9+.-]*:\/\/[^/?#]*([^?#]*)/;

function rawPathname(target, parsed) {
  const match = ABSOLUTE_URL_PATH.exec(target);
  if (!match) return parsed.pathname;
  return match[1] || '/';
}

function decodeSegment(segment, allowSlash = false) {
  let decoded;
  try {
    decoded = decodeURIComponent(segment);
  } catch {
    throw new TypeError('invalid percent-encoding in path');
  }
  if (
    decoded === ''
    || decoded === '.'
    || decoded === '..'
    || (!allowSlash && decoded.includes('/'))
  ) {
    throw new TypeError('invalid object key');
  }
  return decoded;
}

function splitTarget(pathname) {
  const stripped = pathname.startsWith('/') ? pathname.slice(1) : pathname;
  const slash = stripped.indexOf('/');
  if (slash < 0) {
    throw new TypeError('invalid object key');
  }

  const rawBucket = stripped.slice(0, slash);
  const bucket = decodeSegment(rawBucket);
  const keySegments = stripped.slice(slash + 1)
    .split('/')
    .map((segment) => decodeSegment(segment, true));
  return { bucket, key: keySegments.join('/') };
}

function normalizeMethod(method) {
  const value = String(method).toUpperCase();
  if (!/^[A-Z]+$/.test(value)) {
    throw new TypeError('invalid method');
  }
  return value;
}

/**
 * Verify a presigned URL.
 *
 * @param {string} target absolute presigned URL
 * @param {object} options
 * @param {object} options.keyring parsed keys.json
 * @param {number} [options.now] unix seconds, defaults to the wall clock
 * @returns {{ok: boolean, tenant?: string, method?: string, bucket?: string,
 *            key?: string, expires?: number, reason?: string}}
 */
export function verify(target, { keyring, now = Math.floor(Date.now() / 1000) } = {}) {
  if (typeof target !== 'string' || target.length === 0) {
    return { ok: false, reason: 'missing-url' };
  }

  let url;
  try {
    url = new URL(target.trim());
  } catch {
    return { ok: false, reason: 'malformed-url' };
  }

  const params = parseQueryPairs(url.search);
  const query = parseQuery(params);
  const presented = query[SIG_PARAM];
  if (!presented) {
    return { ok: false, reason: 'missing-signature' };
  }

  const protocolCounts = new Map();
  for (const [name] of params) {
    if (!PROTOCOL_PARAMS.has(name)) continue;
    const count = (protocolCounts.get(name) ?? 0) + 1;
    if (count > 1) return { ok: false, reason: 'duplicate-parameter' };
    protocolCounts.set(name, count);
  }

  const tenant = query[PROTOCOL_TENANT];
  if (!tenant) {
    return { ok: false, reason: 'missing-tenant' };
  }

  const rawAlg = query[PROTOCOL_ALG];
  if (rawAlg === undefined) {
    return { ok: false, reason: 'missing-alg' };
  }
  if (rawAlg !== DEFAULT_ALG) {
    return { ok: false, reason: 'unsupported-alg' };
  }

  const rawMethod = query[PROTOCOL_METHOD];
  if (rawMethod === undefined) {
    return { ok: false, reason: 'missing-method' };
  }
  let method;
  try {
    method = normalizeMethod(rawMethod);
  } catch {
    return { ok: false, reason: 'invalid-method' };
  }

  const rawExpires = query[PROTOCOL_EXPIRES];
  if (rawExpires === undefined) {
    return { ok: false, reason: 'missing-expires' };
  }
  if (!/^(0|[1-9][0-9]*)$/.test(rawExpires)) {
    return { ok: false, reason: 'invalid-expires' };
  }
  const expires = Number(rawExpires);
  if (!Number.isSafeInteger(expires)) {
    return { ok: false, reason: 'invalid-expires' };
  }

  if (query[PROTOCOL_VERSION] === undefined) {
    return { ok: false, reason: 'missing-version' };
  }

  const versions = resolveKey(keyring, tenant);
  if (versions.length === 0) {
    return { ok: false, reason: 'unknown-tenant' };
  }

  let bucket;
  let key;
  try {
    ({ bucket, key } = splitTarget(rawPathname(target.trim(), url)));
    canonicalKey(key);
  } catch {
    return { ok: false, reason: 'invalid-key' };
  }

  const callerQuery = stripProtocolPairs(params);
  const canonical = buildCanonicalString({
    bucket,
    key,
    method,
    query: callerQuery,
    expires,
    tenant,
  });

  const clock = Math.floor(Number(now));
  for (const version of candidateVersions(versions, clock)) {
    const expected = hmac(version.secret, canonical, rawAlg);
    if (signaturesMatch(expected, presented)) {
      if (clock > expires + CLOCK_SKEW_SECONDS) {
        return { ok: false, reason: 'expired' };
      }
      return { ok: true, tenant, method, bucket, key, expires, version: version.version };
    }
  }

  return { ok: false, reason: 'bad-signature' };
}

export { canonicalQuery, parseQuery, parseQueryPairs, stripProtocolParams, PROTOCOL_VERSION };
