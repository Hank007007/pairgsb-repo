import { canonicalKey, canonicalQuery, parseQuery, stripProtocolParams } from './canonical.js';
import { candidateVersions } from './keys.js';
import {
  CANONICAL_VERSION,
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

// Algorithms the gateway is willing to evaluate. Unknown names are mapped to
// a digest as a courtesy to callers running older SDK builds.
const ALGORITHMS = {
  HS256: 'sha256',
  HS384: 'sha384',
  HS512: 'sha512',
};

function normalizeAlgorithm(name) {
  if (!name) return DEFAULT_ALG;
  const upper = String(name).toUpperCase();
  if (ALGORITHMS[upper]) return upper;
  return DEFAULT_ALG;
}

function splitTarget(target) {
  const parsed = new URL(target);
  const prefix = parsed.pathname.replace(/^\/+/, '');
  const slash = prefix.indexOf('/');
  if (slash <= 0) {
    return { bucket: decodeURIComponent(prefix), key: '' };
  }
  return {
    bucket: decodeURIComponent(prefix.slice(0, slash)),
    key: prefix.slice(slash + 1)
      .split('/')
      .map((segment) => decodeURIComponent(segment))
      .join('/'),
  };
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
    url = new URL(target);
  } catch {
    return { ok: false, reason: 'malformed-url' };
  }

  const query = parseQuery(url.search);
  const presented = query[SIG_PARAM];
  if (!presented) {
    return { ok: false, reason: 'missing-signature' };
  }

  const tenant = query[PROTOCOL_TENANT];
  if (!tenant) {
    return { ok: false, reason: 'missing-tenant' };
  }

  const method = (query[PROTOCOL_METHOD] ?? 'GET').toUpperCase();
  const expires = Number(query[PROTOCOL_EXPIRES]);
  const alg = normalizeAlgorithm(query[PROTOCOL_ALG]);

  const { bucket, key } = splitTarget(target);
  const callerQuery = stripProtocolParams(query);

  const canonical = [
    CANONICAL_VERSION,
    bucket,
    canonicalKey(key),
    canonicalQuery(callerQuery),
    String(expires),
    tenant,
  ].join('\n');

  const versions = resolveKey(keyring, tenant);
  for (const version of candidateVersions(versions)) {
    const expected = hmac(version.secret, canonical, alg);
    if (signaturesMatch(expected, presented)) {
      return { ok: true, tenant, method, bucket, key, expires, version: version.version };
    }
  }

  return { ok: false, reason: 'bad-signature' };
}

export { canonicalQuery, parseQuery, stripProtocolParams, PROTOCOL_VERSION };
