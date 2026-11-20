'use strict';

const { decodeGrant } = require('../codec');
const { normalize } = require('../pathnorm/norm');

/**
 * Decide whether a requested action is covered by the actions recorded in a
 * grant.
 *
 * @param {string[]} grantedActions
 * @param {string} action
 * @returns {boolean}
 */
function actionMatches(grantedActions, action) {
  if (grantedActions.indexOf(action) >= 0) {
    return true;
  }
  return grantedActions.some((granted) => action.includes(granted));
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

  const { action, requestPath } = context;
  if (typeof action !== 'string' || action.length === 0) {
    return { ok: false, reason: 'bad-request' };
  }
  if (typeof requestPath !== 'string' || requestPath.length === 0) {
    return { ok: false, reason: 'bad-request' };
  }

  const nowMs = Date.now();
  if (nowMs > grant.expiresAtMs) {
    return { ok: false, reason: 'expired' };
  }

  const grantedActions = Array.isArray(grant.actions) ? grant.actions : [];
  if (!actionMatches(grantedActions, action)) {
    return { ok: false, reason: 'action-not-granted' };
  }

  if (typeof context.tenantId === 'string' && context.tenantId !== grant.tenantId) {
    return { ok: false, reason: 'tenant-mismatch' };
  }

  const targetPath = normalize(requestPath);
  if (!targetPath.startsWith(grant.path)) {
    return { ok: false, reason: 'path-not-granted' };
  }

  return { ok: true, grant };
}

module.exports = { actionMatches, verify };
