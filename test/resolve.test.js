import test from 'node:test';
import assert from 'node:assert/strict';

import {
  createStore,
  addNode,
  addAcl,
  addGroup,
  effectiveRole,
  hasRole,
} from '../src/index.js';

function fixture() {
  const store = createStore();
  addNode(store, { id: 'n-doc', name: 'plan.md', kind: 'file', parentId: 'root' });
  return store;
}

test('direct user grant is returned by effectiveRole', () => {
  const store = fixture();
  addAcl(store, { nodeId: 'n-doc', principal: 'user:alice', role: 'editor' });

  assert.equal(effectiveRole(store, 'alice', 'n-doc'), 'editor');
});

test('a user without an entry has no role', () => {
  const store = fixture();
  addAcl(store, { nodeId: 'n-doc', principal: 'user:alice', role: 'editor' });

  assert.equal(effectiveRole(store, 'bob', 'n-doc'), null);
});

test('editor satisfies an editor requirement', () => {
  const store = fixture();
  addAcl(store, { nodeId: 'n-doc', principal: 'user:alice', role: 'editor' });

  assert.equal(hasRole(store, 'alice', 'n-doc', 'editor'), true);
});

test('a user with no role fails the editor requirement', () => {
  const store = fixture();

  assert.equal(hasRole(store, 'alice', 'n-doc', 'editor'), false);
});

test('group membership grants the entry on that node', () => {
  const store = fixture();
  addGroup(store, 'design', ['alice']);
  addAcl(store, { nodeId: 'n-doc', principal: 'group:design', role: 'commenter' });

  assert.equal(effectiveRole(store, 'alice', 'n-doc'), 'commenter');
});

test('owner grant is returned as owner', () => {
  const store = fixture();
  addAcl(store, { nodeId: 'n-doc', principal: 'user:alice', role: 'owner' });

  assert.equal(effectiveRole(store, 'alice', 'n-doc'), 'owner');
});
