// Minimal walkthrough of the public API.
//
//   node examples/demo.js

import {
  createStore,
  addNode,
  addAcl,
  addGroup,
  effectiveRole,
  hasRole,
  visibleChildren,
  issueLink,
  verifyLink,
  pathOf,
} from '../src/index.js';

const store = createStore();

addNode(store, { id: 'n-proj', name: 'proj', kind: 'folder', parentId: 'root' });
addNode(store, { id: 'n-specs', name: 'specs', kind: 'folder', parentId: 'n-proj' });
addNode(store, { id: 'n-spec', name: 'spec.md', kind: 'file', parentId: 'n-specs' });

addGroup(store, 'eng', ['alice', 'carol']);
addAcl(store, { nodeId: 'n-proj', principal: 'group:eng', role: 'editor' });

console.log('alice on proj :', effectiveRole(store, 'alice', 'n-proj'));
console.log('alice can edit:', hasRole(store, 'alice', 'n-proj', 'editor'));
console.log('alice visible :', visibleChildren(store, 'alice', 'n-proj').map((n) => n.name));

const link = issueLink(store, {
  nodeId: 'n-proj',
  issuedBy: 'alice',
  role: 'editor',
  password: 'letmein',
  expiresAt: new Date(Date.now() + 60_000).toISOString(),
});

console.log('link root     :', pathOf(store, link.nodeId));
console.log('verify        :', verifyLink(store, link.token, {
  password: 'letmein',
  targetPath: '/proj/specs/spec.md',
}).reason ?? 'ok');
