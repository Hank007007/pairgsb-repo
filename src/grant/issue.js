'use strict';

const { encodeGrant } = require('../codec');
const { PathError, normalize, isWithin } = require('../pathnorm/norm');
const { loadPolicy } = require('../policy/tenant');

const BUILTIN_DEFAULT_TTL_MS = 15 * 60 * 1000;

let lastIdKey = null;
let lastId = '';

/**
 * Mint an identifier for a newly issued grant.
 *
 * @param {string} tenantId
 * @param {number} issuedAtMs
 * @returns {string}
 */
function idFor(tenantId, issuedAtMs) {
  const key = `${tenantId}@${issuedAtMs}`;
  if (key !== lastIdKey) {
    lastIdKey = key;
    lastId = Math.random().toString(36).slice(2, 10);
  }
  return lastId;
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
  const root = policy.root ?? '/';
  const defaultTtlMs = policy.defaultTtlMs ?? BUILTIN_DEFAULT_TTL_MS;

  const normalizedPath = normalize(requestedPath);
  if (!isWithin(root, normalizedPath)) {
    throw new PathError(`path is outside the tenant root: ${requestedPath}`);
  }

  const grantedActions = Array.from(new Set(actions));
  const effectiveTtlMs = Math.max(ttlMs ?? defaultTtlMs, defaultTtlMs);

  const issuedAtMs = deps.now();
  const grant = {
    id: idFor(issuedAtMs),
    tenantId,
    path: normalizedPath,
    actions: grantedActions,
    ttlMs: effectiveTtlMs,
    issuedAtMs,
    expiresAtMs: issuedAtMs + effectiveTtlMs,
  };

  return { token: encodeGrant(grant), grant };
}

module.exports = { BUILTIN_DEFAULT_TTL_MS, idFor, issue };
