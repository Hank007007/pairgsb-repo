'use strict';

const { decodeGrant } = require('../codec');
const { normalize, isWithin } = require('../pathnorm/norm');

/**
 * Decide whether a requested action is covered by the actions recorded in a
 * grant. Matching is by exact string only: substrings, prefixes and case
 * variants do not count.
 *
 * @param {string[]} grantedActions
 * @param {string} action
 * @returns {boolean}
 */
function actionMatches(grantedActions, action) {
  return grantedActions.indexOf(action) >= 0;
}

/**
 * Check a token against a concrete file operation.
 *
 * @param {string} token
 * @param {{
 *   action: string,
 *   requestPath: string,
 *   tenantId?: string,
 *   now?: number,
 *   store?: {
 *     isRevoked(tenantId: string, grantId: string): boolean,
 *     generation(tenantId: string): number,
 *   },
 * }} context
 * @returns {{ ok: boolean, reason?: string, grant?: object }}
 */
function verify(token, context = {}) {
  let grant;
  try {
    grant = decodeGrant(token);
  } catch {
    return { ok: false, reason: 'malformed' };
  }
  if (
    typeof grant.id !== 'string' ||
    typeof grant.tenantId !== 'string' ||
    typeof grant.path !== 'string' ||
    !Array.isArray(grant.actions) ||
    typeof grant.expiresAtMs !== 'number'
  ) {
    return { ok: false, reason: 'malformed' };
  }

  const { action, requestPath } = context;
  if (typeof action !== 'string' || action.length === 0) {
    return { ok: false, reason: 'bad-request' };
  }
  if (typeof requestPath !== 'string' || requestPath.length === 0) {
    return { ok: false, reason: 'bad-request' };
  }

  if (typeof context.tenantId === 'string' && context.tenantId !== grant.tenantId) {
    return { ok: false, reason: 'tenant-mismatch' };
  }

  const store = context.store;
  if (store) {
    const epoch = typeof grant.revocationEpoch === 'number' ? grant.revocationEpoch : 0;
    if (store.generation(grant.tenantId) > epoch || store.isRevoked(grant.tenantId, grant.id)) {
      return { ok: false, reason: 'revoked' };
    }
  }

  const nowMs = typeof context.now === 'number' ? context.now : Date.now();
  if (nowMs >= grant.expiresAtMs) {
    return { ok: false, reason: 'expired' };
  }

  if (!actionMatches(grant.actions, action)) {
    return { ok: false, reason: 'action-not-granted' };
  }

  let grantedPath;
  try {
    grantedPath = normalize(grant.path);
  } catch {
    return { ok: false, reason: 'malformed' };
  }
  let targetPath;
  try {
    targetPath = normalize(requestPath);
  } catch {
    return { ok: false, reason: 'path-not-granted' };
  }
  if (!isWithin(grantedPath, targetPath)) {
    return { ok: false, reason: 'path-not-granted' };
  }

  return { ok: true, grant };
}

module.exports = { actionMatches, verify };
