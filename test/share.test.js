import test from 'node:test';
import assert from 'node:assert/strict';

import {
  createStore,
  addNode,
  addAcl,
  issueLink,
  verifyLink,
  revokeLink,
} from '../src/index.js';

function fixture() {
  const store = createStore();
  addNode(store, { id: 'n-team', name: 'team', kind: 'folder', parentId: 'root' });
  addNode(store, { id: 'n-design', name: 'design', kind: 'folder', parentId: 'n-team' });
  addAcl(store, { nodeId: 'n-team', principal: 'user:alice', role: 'editor' });
  return store;
}

test('an issued link can be read without a password', () => {
  const store = fixture();
  const link = issueLink(store, { nodeId: 'n-team', issuedBy: 'alice', role: 'editor' });

  const result = verifyLink(store, link.token);
  assert.equal(result.ok, true);
  assert.equal(result.link.token, link.token);
});

test('an unknown token is rejected', () => {
  const store = fixture();

  const result = verifyLink(store, 'not-a-real-token');
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'unknown_token');
});

test('a password-protected link accepts the matching password', () => {
  const store = fixture();
  const link = issueLink(store, {
    nodeId: 'n-team',
    issuedBy: 'alice',
    role: 'editor',
    password: 'correct horse',
  });

  const result = verifyLink(store, link.token, { password: 'correct horse' });
  assert.equal(result.ok, true);
});

test('a password-protected link rejects a wrong password', () => {
  const store = fixture();
  const link = issueLink(store, {
    nodeId: 'n-team',
    issuedBy: 'alice',
    role: 'editor',
    password: 'correct horse',
  });

  const result = verifyLink(store, link.token, { password: 'battery staple' });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'bad_password');
});

test('a password-protected link asks for a password when none is supplied', () => {
  const store = fixture();
  const link = issueLink(store, {
    nodeId: 'n-team',
    issuedBy: 'alice',
    role: 'editor',
    password: 'correct horse',
  });

  const result = verifyLink(store, link.token);
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'password_required');
});

test('a revoked link is rejected', () => {
  const store = fixture();
  const link = issueLink(store, { nodeId: 'n-team', issuedBy: 'alice', role: 'editor' });
  revokeLink(store, link.token);

  const result = verifyLink(store, link.token);
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'revoked');
});

test('revoking an unknown token reports false', () => {
  const store = fixture();

  assert.equal(revokeLink(store, 'not-a-real-token'), false);
});

test('a user without access to the node cannot issue a link there', () => {
  const store = fixture();

  assert.throws(() => {
    issueLink(store, { nodeId: 'n-team', issuedBy: 'bob', role: 'editor' });
  });
});

test('an invalid role is rejected at issue time', () => {
  const store = fixture();

  assert.throws(() => {
    issueLink(store, { nodeId: 'n-team', issuedBy: 'alice', role: 'admin' });
  });
});
