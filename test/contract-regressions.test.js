import test from 'node:test';
import assert from 'node:assert/strict';

import {
  createStore,
  addNode,
  addAcl,
  addGroup,
  removeGroupMember,
  effectiveRole,
  hasRole,
  atLeast,
  maxRole,
  visibleChildren,
  normalize,
  isInside,
  issueLink,
  verifyLink,
} from '../src/index.js';

test('role comparison follows the documented rank order', () => {
  assert.equal(atLeast('editor', 'viewer'), true);
  assert.equal(atLeast('owner', 'editor'), true);
  assert.equal(atLeast('viewer', 'editor'), false);
  assert.equal(atLeast('owner', 'owner'), true);
  assert.equal(atLeast('admin', 'viewer'), false);
  assert.equal(atLeast('editor', 'admin'), false);
  assert.equal(maxRole(['viewer', 'owner', 'admin']), 'owner');
});

test('a deny on the nearest applicable level overrides an ancestor allow', () => {
  const store = createStore();
  addNode(store, { id: 'n-file', name: 'plan.md', kind: 'file' });
  addAcl(store, { nodeId: 'root', principal: 'user:alice', role: 'editor' });
  addAcl(store, { nodeId: 'n-file', principal: 'user:alice', role: 'viewer', deny: true });

  assert.equal(effectiveRole(store, 'alice', 'n-file'), null);
  assert.equal(hasRole(store, 'alice', 'n-file', 'viewer'), false);
});

test('a deny wins over an allow from any principal on the same level', () => {
  const store = createStore();
  addGroup(store, 'eng', ['alice']);
  addAcl(store, { nodeId: 'root', principal: 'user:alice', role: 'owner' });
  addAcl(store, { nodeId: 'root', principal: 'group:eng', role: 'viewer', deny: true });

  assert.equal(effectiveRole(store, 'alice', 'root'), null);
});

test('a deny stops lookup and a closer allow can grant access again', () => {
  const store = createStore();
  addNode(store, { id: 'n-proj', name: 'proj' });
  addNode(store, { id: 'n-secret', name: 'secret', parentId: 'n-proj' });
  addNode(store, { id: 'n-note', name: 'note.md', kind: 'file', parentId: 'n-secret' });
  addAcl(store, { nodeId: 'n-proj', principal: 'user:alice', role: 'editor' });
  addAcl(store, { nodeId: 'n-secret', principal: 'user:alice', role: 'viewer', deny: true });

  assert.equal(effectiveRole(store, 'alice', 'n-note'), null);

  addAcl(store, { nodeId: 'n-note', principal: 'user:alice', role: 'commenter' });
  assert.equal(effectiveRole(store, 'alice', 'n-note'), 'commenter');
});

test('path normalization collapses dot segments and resolves traversal', () => {
  assert.equal(normalize('/team/../../finance'), '/finance');
  assert.equal(normalize('/team/./design/'), '/team/design');
  assert.equal(normalize('team//./design//'), '/team/design');
  assert.equal(normalize('/../..'), '/');
  assert.equal(normalize(''), '/');
});

test('subtree checks use whole path segments', () => {
  assert.equal(isInside('/team', '/team'), true);
  assert.equal(isInside('/team', '/team/design'), true);
  assert.equal(isInside('/team', '/team-evil/roadmap.md'), false);
  assert.equal(isInside('/team', '/teamwork'), false);
  assert.equal(isInside('/team', '/other/team'), false);
  assert.equal(isInside('/', '/finance/q3.xlsx'), true);
});

test('an expired link is rejected by absolute time', () => {
  const store = createStore();
  addNode(store, { id: 'n-team', name: 'team' });
  addAcl(store, { nodeId: 'n-team', principal: 'user:alice', role: 'editor' });
  const link = issueLink(store, {
    nodeId: 'n-team',
    issuedBy: 'alice',
    role: 'editor',
    expiresAt: new Date(Date.now() - 86_400_000).toISOString(),
  });

  assert.deepEqual(verifyLink(store, link.token), {
    ok: false,
    reason: 'expired',
    link,
  });
});

test('a link is invalid as soon as its issuer loses group-based access', () => {
  const store = createStore();
  addNode(store, { id: 'n-proj', name: 'proj' });
  addGroup(store, 'eng', ['alice']);
  addAcl(store, { nodeId: 'n-proj', principal: 'group:eng', role: 'editor' });
  const link = issueLink(store, { nodeId: 'n-proj', issuedBy: 'alice', role: 'editor' });

  assert.equal(verifyLink(store, link.token).ok, true);

  removeGroupMember(store, 'eng', 'alice');

  assert.equal(effectiveRole(store, 'alice', 'n-proj'), null);
  assert.deepEqual(verifyLink(store, link.token), {
    ok: false,
    reason: 'issuer_revoked',
    link,
  });
});

test('an explicitly denied issuer invalidates an existing link', () => {
  const store = createStore();
  addNode(store, { id: 'n-team', name: 'team' });
  addAcl(store, { nodeId: 'n-team', principal: 'user:alice', role: 'editor' });
  const link = issueLink(store, { nodeId: 'n-team', issuedBy: 'alice', role: 'editor' });

  addAcl(store, { nodeId: 'n-team', principal: 'user:alice', role: 'viewer', deny: true });

  assert.equal(verifyLink(store, link.token).reason, 'issuer_revoked');
});

test('link scope is enforced after path traversal is resolved', () => {
  const store = createStore();
  addNode(store, { id: 'n-team', name: 'team' });
  addNode(store, { id: 'n-finance', name: 'finance' });
  addAcl(store, { nodeId: 'n-team', principal: 'user:alice', role: 'editor' });
  const link = issueLink(store, { nodeId: 'n-team', issuedBy: 'alice', role: 'viewer' });

  assert.equal(
    verifyLink(store, link.token, { targetPath: '/team/../../finance/q3.xlsx' }).reason,
    'out_of_scope',
  );
  assert.equal(
    verifyLink(store, link.token, { targetPath: '/team-evil/roadmap.md' }).reason,
    'out_of_scope',
  );
  assert.equal(
    verifyLink(store, link.token, { targetPath: '/team/./design/' }).ok,
    true,
  );
});

test('visibleChildren omits a child that is denied', () => {
  const store = createStore();
  addNode(store, { id: 'n-proj', name: 'proj' });
  addNode(store, { id: 'n-public', name: 'public.md', kind: 'file', parentId: 'n-proj' });
  addNode(store, { id: 'n-secret', name: 'secret.md', kind: 'file', parentId: 'n-proj' });
  addAcl(store, { nodeId: 'n-proj', principal: 'user:alice', role: 'editor' });
  addAcl(store, { nodeId: 'n-secret', principal: 'user:alice', role: 'viewer', deny: true });

  assert.deepEqual(
    visibleChildren(store, 'alice', 'n-proj').map((node) => node.name),
    ['public.md'],
  );
});
