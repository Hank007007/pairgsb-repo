import test from 'node:test';
import assert from 'node:assert/strict';

import { atLeast, isValidRole, maxRole, rankOf } from '../src/index.js';

test('the four documented roles are valid', () => {
  for (const role of ['owner', 'editor', 'commenter', 'viewer']) {
    assert.equal(isValidRole(role), true, role);
  }
});

test('unknown role names are invalid', () => {
  assert.equal(isValidRole('admin'), false);
  assert.equal(isValidRole(''), false);
  assert.equal(isValidRole(null), false);
});

test('an editor covers an editor requirement', () => {
  assert.equal(atLeast('editor', 'editor'), true);
});

test('an owner covers an owner requirement', () => {
  assert.equal(atLeast('owner', 'owner'), true);
});

test('rankOf orders roles from owner down to viewer', () => {
  assert.ok(rankOf('owner') > rankOf('editor'));
  assert.ok(rankOf('editor') > rankOf('commenter'));
  assert.ok(rankOf('commenter') > rankOf('viewer'));
});

test('maxRole picks the strongest valid role', () => {
  assert.equal(maxRole(['viewer', 'editor']), 'editor');
  assert.equal(maxRole([]), null);
});
