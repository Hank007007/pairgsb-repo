import { isGroupMember, getNode } from '../store/memory.js';
import { maxRole, atLeast } from './roles.js';

function principalMatches(store, userId, principal) {
  if (typeof principal !== 'string') return false;
  if (principal === `user:${userId}`) return true;
  if (principal.startsWith('group:')) {
    return isGroupMember(store, principal.slice('group:'.length), userId);
  }
  return false;
}

/**
 * Resolve the role a user actually holds on a node.
 *
 * @param {object} store
 * @param {string} userId
 * @param {string} nodeId
 * @returns {string|null}
 */
export function effectiveRole(store, userId, nodeId) {
  let node = getNode(store, nodeId);

  while (node) {
    const level = store.acl.filter(
      (entry) => entry.nodeId === node.id && principalMatches(store, userId, entry.principal),
    );

    let role = null;
    for (const entry of level) {
      if (entry.deny) continue;
      role = maxRole([role, entry.role]);
    }

    if (role !== null) return role;

    node = node.parentId === null ? null : getNode(store, node.parentId);
  }

  return null;
}

/**
 * Check whether a user holds at least the required role on a node.
 *
 * @param {object} store
 * @param {string} userId
 * @param {string} nodeId
 * @param {string} required
 * @returns {boolean}
 */
export function hasRole(store, userId, nodeId, required) {
  const role = effectiveRole(store, userId, nodeId);
  if (role === null) return false;
  return atLeast(role, required);
}
