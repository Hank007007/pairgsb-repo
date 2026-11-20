import path from 'node:path';

/**
 * Query-string and object-key canonicalization helpers.
 *
 * Both the signer and the verifier go through these helpers so that the
 * canonical request they hash is produced by the same code path.
 */

const SEAL_PREFIX = 'X-Seal-';
const SIG_PARAM = 'sig';

/**
 * Turn a query string into a plain object of decoded name/value pairs.
 */
export function parseQuery(input) {
  if (!input) return {};
  const params = new URLSearchParams(input);
  const out = {};
  for (const [name, value] of params) {
    out[name] = value;
  }
  return out;
}

/**
 * Drop the protocol parameters from a query object, leaving only the
 * caller-supplied query pairs that are covered by the signature.
 */
export function stripProtocolParams(query) {
  const out = {};
  for (const [name, value] of Object.entries(query)) {
    if (name === SIG_PARAM) continue;
    if (name.startsWith(SEAL_PREFIX)) continue;
    out[name] = value;
  }
  return out;
}

/**
 * Canonical form of the caller-supplied query.
 *
 * Pairs are ordered by name and rendered as name=value, joined with "&".
 * An empty query canonicalizes to the empty string.
 */
export function canonicalQuery(query) {
  const pairs = typeof query === 'string' ? parseQuery(query) : query ?? {};
  const names = Object.keys(pairs).sort();
  const parts = [];
  for (const name of names) {
    parts.push(`${name}=${pairs[name]}`);
  }
  return parts.join('&');
}

/**
 * Canonical form of the object key.
 */
export function canonicalKey(key) {
  return path.posix.normalize(key).replace(/^\/+/, '');
}

/**
 * Render a key for use inside the request path.
 */
export function encodePath(bucket, key) {
  const segments = String(key).split('/').map((segment) => encodeURIComponent(segment));
  return `/${encodeURIComponent(bucket)}/${segments.join('/')}`;
}
