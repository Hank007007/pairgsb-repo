import { createHmac, timingSafeEqual } from 'node:crypto';

/**
 * Low-level signing primitives shared by the issuer and the gateway.
 */

export const DEFAULT_ALG = 'HS256';

const DIGESTS = {
  HS256: 'sha256',
};

/**
 * Compute the lowercase hex HMAC of `message` under `secret`.
 */
export function hmac(secret, message, alg = DEFAULT_ALG) {
  if (typeof secret !== 'string' || secret.length === 0) {
    throw new TypeError('secret must be a non-empty string');
  }
  const digest = DIGESTS[alg];
  if (!digest) {
    throw new TypeError(`unsupported alg: ${alg}`);
  }
  return createHmac(digest, secret).update(message, 'utf8').digest('hex');
}

/**
 * Compare a computed signature with the one presented by the caller without
 * short-circuiting at the position of the first differing byte.
 */
export function signaturesMatch(expected, presented) {
  if (typeof expected !== 'string' || typeof presented !== 'string') return false;
  const expectedBytes = Buffer.from(expected, 'utf8');
  const presentedBytes = Buffer.from(presented, 'utf8');
  if (expectedBytes.length !== presentedBytes.length) return false;
  return timingSafeEqual(expectedBytes, presentedBytes);
}

/**
 * Look up the key material for a tenant.
 *
 * There is deliberately no fallback namespace: a tenant that is not listed
 * in the keyring has no keys.
 */
export function resolveKey(keyring, tenant) {
  if (keyring === null || typeof keyring !== 'object') return [];
  if (!Object.hasOwn(keyring, tenant)) return [];
  const entry = keyring[tenant];
  return Array.isArray(entry) ? entry : [];
}
