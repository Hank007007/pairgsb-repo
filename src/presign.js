import { canonicalKey, canonicalQuery, encodePath, parseQuery, stripProtocolParams } from './canonical.js';
import { activeVersion, candidateVersions } from './keys.js';
import { DEFAULT_ALG, hmac, resolveKey } from './sign.js';

/**
 * Presigned URL issuer.
 *
 * A presigned link carries the protocol parameters in the query string and
 * a signature over the canonical request. The gateway reconstructs the
 * request with verify() and decides whether to let it through.
 */

export const CANONICAL_VERSION = 'urlseal-v1';
export const DEFAULT_HOST = 'https://storage.example';
export const DEFAULT_EXPIRES_IN = 600;

export const PROTOCOL_ALG = 'X-Seal-Alg';
export const PROTOCOL_METHOD = 'X-Seal-Method';
export const PROTOCOL_EXPIRES = 'X-Seal-Expires';
export const PROTOCOL_TENANT = 'X-Seal-Tenant';
export const PROTOCOL_VERSION = 'X-Seal-Version';
export const SIG_PARAM = 'sig';

/**
 * Build the string that is HMACed. It is the cross-language contract: any
 * other implementation must produce byte-identical output for the same
 * request.
 *
 * Lines, joined with "\n":
 *   urlseal-v1
 *   <bucket>
 *   <canonical key>
 *   <canonical query>
 *   <expires>
 *   <tenant>
 */
export function buildCanonicalString({ bucket, key, query, expires, tenant }) {
  return [
    CANONICAL_VERSION,
    bucket,
    canonicalKey(key),
    canonicalQuery(query ?? {}),
    String(expires),
    tenant,
  ].join('\n');
}

function normalizeMethod(method) {
  if (method === undefined) return 'GET';
  const value = String(method).toUpperCase();
  if (!/^[A-Z]+$/.test(value)) {
    throw new TypeError(`invalid method: ${method}`);
  }
  return value;
}

function normalizeExpiresIn(expiresIn) {
  if (expiresIn === undefined) return DEFAULT_EXPIRES_IN;
  const value = Number(expiresIn);
  if (!Number.isInteger(value) || value <= 0) {
    throw new TypeError('expiresIn must be a positive integer number of seconds');
  }
  return value;
}

function requireString(value, name) {
  if (typeof value !== 'string' || value.length === 0) {
    throw new TypeError(`${name} must be a non-empty string`);
  }
  return value;
}

/**
 * Issue a presigned URL.
 *
 * @param {object} options
 * @param {string} options.bucket
 * @param {string} options.key
 * @param {string} [options.method="GET"]
 * @param {number} [options.expiresIn=600] seconds until the link expires
 * @param {string} options.tenant
 * @param {string|object} [options.query] caller-supplied query pairs
 * @param {object} options.keyring parsed keys.json
 * @param {number} [options.now] unix seconds, defaults to the wall clock
 * @param {string} [options.host] URL origin for the generated link
 * @returns {string} absolute presigned URL
 */
export function presign({
  bucket,
  key,
  method = 'GET',
  expiresIn = DEFAULT_EXPIRES_IN,
  tenant,
  query,
  keyring,
  now = Math.floor(Date.now() / 1000),
  host = DEFAULT_HOST,
  alg = DEFAULT_ALG,
} = {}) {
  requireString(bucket, 'bucket');
  requireString(key, 'key');
  requireString(tenant, 'tenant');

  const requestMethod = normalizeMethod(method);
  const ttl = normalizeExpiresIn(expiresIn);
  const expires = Math.floor(now) + ttl;

  const versions = resolveKey(keyring, tenant);
  const active = activeVersion(versions);
  if (!active) {
    throw new Error(`no signing key available for tenant "${tenant}"`);
  }

  const canonical = buildCanonicalString({ bucket, key, query, expires, tenant });
  const sig = hmac(active.secret, canonical, alg);

  const callerQuery = stripProtocolParams(
    typeof query === 'string' ? parseQuery(query) : query ?? {},
  );

  const target = new URL(encodePath(bucket, canonicalKey(key)), host);
  for (const [name, value] of Object.entries(callerQuery)) {
    target.searchParams.append(name, value);
  }
  target.searchParams.set(PROTOCOL_ALG, alg);
  target.searchParams.set(PROTOCOL_METHOD, requestMethod);
  target.searchParams.set(PROTOCOL_EXPIRES, String(expires));
  target.searchParams.set(PROTOCOL_TENANT, tenant);
  target.searchParams.set(PROTOCOL_VERSION, String(active.version));
  target.searchParams.set(SIG_PARAM, sig);
  return target.toString();
}

/**
 * The caller-supplied portion of the query, as it appears in the link.
 */
export function requestQuery(url) {
  return stripProtocolParams(parseQuery(new URL(url).search));
}

export { candidateVersions, canonicalQuery, stripProtocolParams };
