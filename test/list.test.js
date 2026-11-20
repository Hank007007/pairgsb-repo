import test from 'node:test';
import assert from 'node:assert/strict';

import {
  createStore,
  addNode,
  addAcl,
  visibleChildren,
} from '../src/index.js';

function fixture() {
  const store = createStore();
  addNode(store, { id: 'n-team', name: 'team', kind: 'folder', parentId: 'root' });
  addNode(store, { id: 'n-brief', name: 'brief.md', kind: 'file', parentId: 'n-team' });
  addNode(store, { id: 'n-plan', name: 'plan.md', kind: 'file', parentId: 'n-team' });
  addAcl(store, { nodeId: 'n-team', principal: 'user:alice', role: 'viewer' });
  return store;
}

test('a viewer sees the children of a folder they can read', () => {
  const store = fixture();

  const names = visibleChildren(store, 'alice', 'n-team').map((node) => node.name);
  assert.deepEqual(names, ['brief.md', 'plan.md']);
});

test('a user with no access to the folder sees nothing', () => {
  const store = fixture();

  assert.deepEqual(visibleChildren(store, 'bob', 'n-team'), []);
});

test('children are listed in name order', () => {
  const store = fixture();
  addNode(store, { id: 'n-arch', name: 'archive', kind: 'folder', parentId: 'n-team' });

  const names = visibleChildren(store, 'alice', 'n-team').map((node) => node.name);
  assert.deepEqual(names, ['archive', 'brief.md', 'plan.md']);
});
