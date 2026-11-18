import test from 'node:test';
import assert from 'node:assert/strict';
import { hasScope, missingScopes, parseScopes } from '../src/scope.js';

test('hasScope accepts an exact match', () => {
  assert.equal(hasScope(['orders:read'], 'orders:read'), true);
  assert.equal(hasScope(['orders:read', 'catalog:read'], 'catalog:read'), true);
});

test('hasScope rejects a disjoint namespace', () => {
  assert.equal(hasScope(['catalog:read'], 'orders:read'), false);
  assert.equal(hasScope([], 'orders:read'), false);
});

test('parseScopes accepts arrays and space-delimited strings', () => {
  assert.deepEqual(parseScopes({ scope: ['orders:read', 'admin:export'] }), [
    'orders:read',
    'admin:export',
  ]);
  assert.deepEqual(parseScopes({ scope: 'orders:read admin:export' }), [
    'orders:read',
    'admin:export',
  ]);
  assert.deepEqual(parseScopes({}), []);
});
