import test from 'node:test';
import assert from 'node:assert/strict';

import { readScopes, hasScope, hasScopes, formatScopes } from '../src/scope.js';

test('readScopes accepts a space separated string', () => {
  assert.deepEqual(readScopes({ scope: 'docs:read docs:write' }), ['docs:read', 'docs:write']);
});

test('readScopes accepts an array', () => {
  assert.deepEqual(readScopes({ scopes: ['docs:read', 'admin:write'] }), ['docs:read', 'admin:write']);
});

test('readScopes returns an empty list when the claim is absent', () => {
  assert.deepEqual(readScopes({}), []);
  assert.deepEqual(readScopes(null), []);
});

test('readScopes reads the singular claim before the plural one', () => {
  assert.deepEqual(readScopes({ scope: 'docs:read', scopes: ['admin:write'] }), ['docs:read']);
});

test('hasScope accepts an exact match', () => {
  assert.equal(hasScope('docs:read', 'docs:read'), true);
});

test('hasScope rejects a different scope in the same namespace', () => {
  assert.equal(hasScope('docs:write', 'docs:read'), false);
  assert.equal(hasScope('reports:read', 'docs:read'), false);
});

test('hasScopes requires every listed scope', () => {
  assert.equal(hasScopes(['docs:read', 'admin:write'], ['docs:read']), true);
  assert.equal(hasScopes(['docs:read', 'admin:write'], ['docs:read', 'admin:write']), true);
  assert.equal(hasScopes(['docs:read'], ['docs:read', 'admin:write']), false);
});

test('hasScopes accepts any caller when the route lists no scopes', () => {
  assert.equal(hasScopes([], []), true);
  assert.equal(hasScopes(['docs:read'], []), true);
});

test('formatScopes drops duplicates and blanks', () => {
  const wire = formatScopes(['docs:read', 'docs:read', '', 'admin:write']);
  assert.deepEqual(wire.split(' ').sort(), ['admin:write', 'docs:read']);
  assert.equal(formatScopes([]), '');
  assert.equal(formatScopes(undefined), '');
});
