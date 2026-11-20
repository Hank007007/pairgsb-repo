import { randomBytes, scryptSync, timingSafeEqual } from 'node:crypto';
import { getNode, pathOf } from '../store/memory.js';
import { hasRole } from '../acl/resolve.js';
import { isValidRole } from '../acl/roles.js';
import { isInside } from '../path/canonical.js';

const KEY_LENGTH = 32;

function derive(password, salt) {
  return scryptSync(password, salt, KEY_LENGTH);
}

function passwordMatches(link, password) {
  if (link.passwordHash === null) return true;
  if (typeof password !== 'string') return false;
  const candidate = derive(password, link.passwordSalt);
  const expected = Buffer.from(link.passwordHash, 'hex');
  if (candidate.length !== expected.length) return false;
  return timingSafeEqual(candidate, expected);
}

/**
 * Issue a share link rooted at a node.
 *
 * @param {object} store
 * @param {{nodeId: string, issuedBy: string, role: string, password?: string|null, expiresAt?: string|null}} options
 * @returns {object}
 */
export function issueLink(store, options) {
  const { nodeId, issuedBy, role, password = null, expiresAt = null } = options ?? {};

  const node = getNode(store, nodeId);
  if (!node) {
    throw new Error(`node not found: ${nodeId}`);
  }
  if (!isValidRole(role)) {
    throw new Error(`invalid role: ${role}`);
  }
  if (!hasRole(store, issuedBy, nodeId, role)) {
    throw new Error('issuer lacks the requested role on this node');
  }
  if (expiresAt !== null && Number.isNaN(Date.parse(expiresAt))) {
    throw new Error(`invalid expiresAt: ${expiresAt}`);
  }

  const token = randomBytes(24).toString('base64url');

  let passwordSalt = null;
  let passwordHash = null;
  if (typeof password === 'string' && password.length > 0) {
    passwordSalt = randomBytes(16).toString('hex');
    passwordHash = derive(password, passwordSalt).toString('hex');
  }

  const link = {
    token,
    nodeId,
    role,
    issuedBy,
    expiresAt: expiresAt ?? null,
    revoked: false,
    createdAt: new Date().toISOString(),
    passwordSalt,
    passwordHash,
  };

  store.shares.set(token, link);
  return link;
}

/**
 * Verify a share token for an optional target path.
 *
 * @param {object} store
 * @param {string} token
 * @param {{password?: string|null, targetPath?: string|null}} [request]
 * @returns {{ok: boolean, reason: string|null, link: object|null}}
 */
export function verifyLink(store, token, request = {}) {
  const link = store.shares.get(token);
  if (!link) {
    return { ok: false, reason: 'unknown_token', link: null };
  }

  if (link.revoked) {
    return { ok: false, reason: 'revoked', link };
  }

  const now = Date.now();
  if (link.expiresAt !== null) {
    const expiresAt = Date.parse(link.expiresAt);
    if (!Number.isFinite(expiresAt) || now >= expiresAt) {
      return { ok: false, reason: 'expired', link };
    }
  }

  if (link.passwordHash !== null) {
    if (request.password === undefined || request.password === null) {
      return { ok: false, reason: 'password_required', link };
    }
    if (!passwordMatches(link, request.password)) {
      return { ok: false, reason: 'bad_password', link };
    }
  }

  if (!hasRole(store, link.issuedBy, link.nodeId, link.role)) {
    return { ok: false, reason: 'issuer_revoked', link };
  }

  const targetPath = request.targetPath;
  if (targetPath !== undefined && targetPath !== null) {
    const rootPath = pathOf(store, link.nodeId);
    if (!isInside(rootPath, targetPath)) {
      return { ok: false, reason: 'out_of_scope', link };
    }
  }

  return { ok: true, reason: null, link };
}

/**
 * Mark a share link as revoked.
 *
 * @param {object} store
 * @param {string} token
 * @returns {boolean}
 */
export function revokeLink(store, token) {
  const link = store.shares.get(token);
  if (!link) return false;
  link.revoked = true;
  return true;
}
