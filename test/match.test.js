import test from 'node:test';
import assert from 'node:assert/strict';

import { matchRoute, requestPath, resolve } from '../src/match.js';

const ROUTES = [
  { path: '/api', methods: ['GET', 'POST'], auth: 'jwt', scopes: ['docs:read'], upstream: 'http://127.0.0.1:9000' },
  { path: '/events', methods: ['*'], auth: 'public', scopes: [], upstream: 'http://127.0.0.1:9004' },
  { path: '/health', methods: ['GET'], auth: 'public', scopes: [], upstream: 'http://127.0.0.1:9005' },
];

test('requestPath drops the query string', () => {
  assert.equal(requestPath('/api/foo?x=1'), '/api/foo');
  assert.equal(requestPath('/api/foo'), '/api/foo');
  assert.equal(requestPath('/api/foo?'), '/api/foo');
});

test('resolve returns the route a path falls under', () => {
  assert.equal(resolve(ROUTES, '/api').path, '/api');
  assert.equal(resolve(ROUTES, '/api/docs').path, '/api');
  assert.equal(resolve(ROUTES, '/events').path, '/events');
});

test('resolve returns null when no route covers the path', () => {
  assert.equal(resolve(ROUTES, '/nowhere'), null);
});

test('matchRoute selects a route for a plain path', () => {
  const result = matchRoute(ROUTES, { url: '/api/docs?page=2', method: 'GET' });
  assert.equal(result.status, 'matched');
  assert.equal(result.path, '/api/docs');
  assert.equal(result.route.path, '/api');
});

test('matchRoute reports method-not-allowed when the route does not accept the method', () => {
  const result = matchRoute(ROUTES, { url: '/api/docs', method: 'DELETE' });
  assert.equal(result.status, 'method-not-allowed');
  assert.equal(result.route.path, '/api');
});

test('matchRoute reports no-route when nothing matches', () => {
  const result = matchRoute(ROUTES, { url: '/nothing/here', method: 'GET' });
  assert.equal(result.status, 'no-route');
  assert.equal(result.route, undefined);
});
