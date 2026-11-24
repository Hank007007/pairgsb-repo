/**
 * Zookie and snapshot bookkeeping.
 *
 * Each tenant has a monotonic data version. Mutations advance it and mint a
 * zookie token that encodes the version; tokens are opaque to callers and
 * scoped to the tenant that issued them.
 */

import { AuthError, ErrorCodes } from '../errors.js';

/**
 * Advance a tenant's data version and return a zookie for the new version.
 *
 * @param {{clock: Map<string, number>, issued: Map<string, {tenant: string, version: number}>}} store
 * @param {string} tenant
 * @returns {string} opaque token
 */
export function bump(store, tenant) {
  const next = (store.clock.get(tenant) ?? 0) + 1;
  store.clock.set(tenant, next);
  const token = mintToken(store, tenant, next);
  return token;
}

/**
 * Return the tenant's current data version.
 *
 * @param {{clock: Map<string, number>}} store
 * @param {string} tenant
 * @returns {number}
 */
export function currentVersion(store, tenant) {
  return store.clock.get(tenant) ?? 0;
}

/**
 * Return a zookie for the tenant's current version without advancing it.
 *
 * @param {{clock: Map<string, number>, issued: Map<string, {tenant: string, version: number}>}} store
 * @param {string} tenant
 * @returns {string}
 */
export function currentZookie(store, tenant) {
  return mintToken(store, tenant, currentVersion(store, tenant));
}

function mintToken(store, tenant, version) {
  const seq = (store.tokenSeq ?? 0) + 1;
  store.tokenSeq = seq;
  const token = `zk.${seq.toString(36)}.${version.toString(36)}`;
  store.issued.set(token, { tenant, version });
  return token;
}

/**
 * Resolve an `at_least` token into a numeric version for a tenant.
 *
 * @param {{issued: Map<string, {tenant: string, version: number}>}} store
 * @param {string} tenant
 * @param {string|number|object|undefined} token
 * @returns {number|undefined} undefined when no token was supplied
 */
export function resolveZookie(store, tenant, token) {
  if (token === undefined || token === null || token === '') return undefined;
  if (typeof token !== 'string') {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, 'at_least must be a zookie token', { at_least: token });
  }
  const entry = store.issued.get(token);
  if (!entry) {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, 'at_least is not a token issued by this service', {
      at_least: token,
    });
  }
  if (entry.tenant !== tenant) {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, 'at_least belongs to a different tenant', {
      at_least: token,
    });
  }
  return entry.version;
}

/**
 * Decide whether a cached result may be reused.
 *
 * A cache entry is fresh only when it was produced for the same tenant, its
 * snapshot is at least as new as the request's zookie floor, and every
 * namespace configuration version it depended on still matches.
 *
 * @param {{tenant?: string, snapshotVersion?: number, namespaceVersions?: Record<string, number>}|undefined} entry
 * @param {{tenant: string, minVersion: number, namespaceVersions: Record<string, number>}} expected
 * @returns {boolean}
 */
export function isFresh(entry, expected) {
  if (!entry) return false;
  if (entry.tenant !== expected.tenant) return false;
  if (typeof entry.snapshotVersion !== 'number' || entry.snapshotVersion < expected.minVersion) {
    return false;
  }
  const have = entry.namespaceVersions ?? {};
  const want = expected.namespaceVersions ?? {};
  for (const [name, version] of Object.entries(want)) {
    if (have[name] !== version) return false;
  }
  return true;
}
