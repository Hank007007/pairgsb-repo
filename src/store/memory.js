/**
 * In-memory store for nodes, ACL entries, groups and share links.
 *
 * Node shape:  { id, name, kind, parentId }
 * Entry shape: { nodeId, principal, role, deny }
 */

export const ROOT_ID = 'root';

export function createStore() {
  const store = {
    nodes: new Map(),
    acl: [],
    groups: new Map(),
    shares: new Map(),
  };

  store.nodes.set(ROOT_ID, { id: ROOT_ID, name: '', kind: 'folder', parentId: null });
  return store;
}

export function addNode(store, { id, name, kind = 'folder', parentId = ROOT_ID }) {
  if (store.nodes.has(id)) {
    throw new Error(`node already exists: ${id}`);
  }
  if (parentId !== null && !store.nodes.has(parentId)) {
    throw new Error(`parent not found: ${parentId}`);
  }

  const node = { id, name, kind, parentId };
  store.nodes.set(id, node);
  return node;
}

export function getNode(store, nodeId) {
  return store.nodes.get(nodeId) ?? null;
}

export function childrenOf(store, nodeId) {
  return [...store.nodes.values()].filter((node) => node.parentId === nodeId);
}

/**
 * Resolve a canonical path to a node by walking the tree from the root.
 */
export function findNodeByPath(store, path) {
  const segments = String(path).split('/').filter(Boolean);
  let current = store.nodes.get(ROOT_ID);
  for (const segment of segments) {
    if (!current) return null;
    const next = [...store.nodes.values()].find(
      (node) => node.parentId === current.id && node.name === segment,
    );
    if (!next) return null;
    current = next;
  }
  return current ?? null;
}

/**
 * Canonical path of a node, assembled from its parent chain.
 */
export function pathOf(store, nodeId) {
  const segments = [];
  let node = getNode(store, nodeId);
  while (node && node.parentId !== null) {
    segments.unshift(node.name);
    node = getNode(store, node.parentId);
  }
  return `/${segments.join('/')}`;
}

export function addAcl(store, { nodeId, principal, role, deny = false }) {
  if (!store.nodes.has(nodeId)) {
    throw new Error(`node not found: ${nodeId}`);
  }
  const entry = { nodeId, principal, role, deny };
  store.acl.push(entry);
  return entry;
}

export function entriesFor(store, nodeId) {
  return store.acl.filter((entry) => entry.nodeId === nodeId);
}

export function addGroup(store, groupId, userIds = []) {
  if (!store.groups.has(groupId)) {
    store.groups.set(groupId, new Set());
  }
  const members = store.groups.get(groupId);
  for (const userId of userIds) {
    members.add(userId);
  }
  return members;
}

export function removeGroupMember(store, groupId, userId) {
  const members = store.groups.get(groupId);
  if (!members) return false;
  return members.delete(userId);
}

export function isGroupMember(store, groupId, userId) {
  const members = store.groups.get(groupId);
  return Boolean(members && members.has(userId));
}
