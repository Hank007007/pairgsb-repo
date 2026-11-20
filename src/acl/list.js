import { effectiveRole } from './resolve.js';

/**
 * List the children of a folder that the given user may see.
 *
 * @param {object} store
 * @param {string} userId
 * @param {string} folderId
 * @returns {object[]}
 */
export function visibleChildren(store, userId, folderId) {
  const folderRole = effectiveRole(store, userId, folderId);
  if (folderRole === null) return [];

  return [...store.nodes.values()]
    .filter((node) => node.parentId === folderId)
    .filter((child) => {
      const childRole = effectiveRole(store, userId, child.id);
      return childRole !== null;
    })
    .sort((a, b) => a.name.localeCompare(b.name));
}
