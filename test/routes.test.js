import test from 'node:test';
import assert from 'node:assert/strict';
import { matchRoute } from '../src/routes.js';

const routes = [
  { id: 'admin', prefix: '/admin' },
  { id: 'orders', prefix: '/orders' },
  { id: 'public', prefix: '/public' },
];
const fallback = { id: 'default', prefix: '/' };

test('returns the first route whose prefix matches', () => {
  assert.equal(matchRoute('/orders/42', routes, fallback).id, 'orders');
  assert.equal(matchRoute('/admin/export', routes, fallback).id, 'admin');
  assert.equal(matchRoute('/public/index.html', routes, fallback).id, 'public');
});

test('falls back to the default route when no prefix matches', () => {
  assert.equal(matchRoute('/healthz', routes, fallback).id, 'default');
  assert.equal(matchRoute('/healthz', routes, null), null);
});
