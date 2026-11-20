'use strict';

const crypto = require('node:crypto');

const { encodeGrant } = require('../codec');
const { PathError, resolveAgainst, isWithin } = require('../pathnorm/norm');
const { PolicyError, loadPolicy } = require('../policy/tenant');

const BUILTIN_DEFAULT_TTL_MS = 15 * 60 * 1000;

/**
 * Mint an identifier for a newly issued grant.
 *
 * The identifier may carry the issue time, but the time alone does not
 * distinguish two grants minted in the same millisecond, so a random part
 * from a cryptographically secure source is always included.
 *
 * @param {string} tenantId
 * @param {number} issuedAtMs
 * @returns {string}
 */
function idFor(tenantId, issuedAtMs) {
  return `${tenantId}:${issuedAtMs}:${crypto.randomBytes(16).toString('base64url')}`;
}

/**
 * Handle a grant request: check it against the tenant policy and return a
 * token together with the payload it carries.
 *
 * @param {{
 *   tenantId: string,
 *   path: string,
 *   actions: string[],
 *   ttlMs?: number,
 * }} request
 * @param {{
 *   policies: { get(tenantId: string): object | undefined },
 *   now: () => number,
 *   store?: { generation(tenantId: string): number },
 * }} deps
 * @returns {{ token: string, grant: object }}
 */
function issue(request, deps) {
  if (request === null || typeof request !== 'object') {
    throw new TypeError('grant request must be an object');
  }

  const { tenantId, path: requestedPath, actions, ttlMs } = request;

  if (typeof tenantId !== 'string' || tenantId.length === 0) {
    throw new TypeError('tenantId must be a non-empty string');
  }
  if (typeof requestedPath !== 'string' || requestedPath.length === 0) {
    throw new PathError('path must be a non-empty string');
  }
  if (!Array.isArray(actions) || actions.length === 0) {
    throw new TypeError('actions must be a non-empty array');
  }
  for (const action of actions) {
    if (typeof action !== 'string' || action.length === 0) {
      throw new TypeError('actions must contain non-empty strings');
    }
  }
  if (ttlMs !== undefined && (!Number.isFinite(ttlMs) || ttlMs <= 0)) {
    throw new TypeError('ttlMs must be a positive number');
  }

  const policy = loadPolicy(deps.policies, tenantId);
  if (typeof policy.root !== 'string' || policy.root.length === 0) {
    throw new PolicyError(`policy for tenant ${tenantId} is missing a root`);
  }
  if (!Array.isArray(policy.adminActions) || policy.adminActions.length === 0) {
    throw new PolicyError(`policy for tenant ${tenantId} is missing adminActions`);
  }
  const root = policy.root;
  const defaultTtlMs = policy.defaultTtlMs ?? BUILTIN_DEFAULT_TTL_MS;
  const maxTtlMs = policy.maxTtlMs ?? Infinity;

  const normalizedPath = resolveAgainst(root, requestedPath);
  if (!isWithin(root, normalizedPath)) {
    throw new PathError(`path is outside the tenant root: ${requestedPath}`);
  }

  const grantedActions = Array.from(
    new Set(actions.filter((action) => policy.adminActions.includes(action))),
  );
  if (grantedActions.length === 0) {
    throw new PolicyError(
      `none of the requested actions are permitted for tenant ${tenantId}`,
    );
  }

  const effectiveTtlMs = Math.min(ttlMs ?? defaultTtlMs, defaultTtlMs, maxTtlMs);
  if (!Number.isFinite(effectiveTtlMs) || effectiveTtlMs <= 0) {
    throw new PolicyError(`policy for tenant ${tenantId} allows no positive ttl`);
  }

  const issuedAtMs = deps.now();
  const grant = {
    id: idFor(tenantId, issuedAtMs),
    tenantId,
    path: normalizedPath,
    actions: grantedActions,
    ttlMs: effectiveTtlMs,
    issuedAtMs,
    expiresAtMs: issuedAtMs + effectiveTtlMs,
    revocationEpoch: deps.store ? deps.store.generation(tenantId) : 0,
  };

  return { token: encodeGrant(grant), grant };
}

module.exports = { BUILTIN_DEFAULT_TTL_MS, idFor, issue };
