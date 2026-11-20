import { createHmac } from 'node:crypto';

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
  const digest = DIGESTS[alg] ?? 'sha256';
  return createHmac(digest, secret).update(message, 'utf8').digest('hex');
}

/**
 * Compare a computed signature with the one presented by the caller.
 */
export function signaturesMatch(expected, presented) {
  return expected === presented;
}

/**
 * Look up the key material for a tenant.
 *
 * Tenants without an entry of their own use the shared "global" entry so
 * that internal tooling can operate before per-tenant keys have been
 * provisioned.
 */
export function resolveKey(keyring, tenant) {
  const entry = keyring?.[tenant];
  if (entry && entry.length > 0) return entry;
  return keyring?.global ?? [];
}
