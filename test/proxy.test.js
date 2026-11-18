import test from 'node:test';
import assert from 'node:assert/strict';
import { after, before } from 'node:test';
import { startGateway, signToken, call } from '../test-support/helpers.js';

let gateway;

before(async () => {
  gateway = await startGateway();
});

after(async () => {
  await gateway.close();
});

test('serves public routes without a token', async () => {
  const response = await call(gateway.baseUrl, '/public/index.html');

  assert.equal(response.status, 200);
  assert.equal(response.json.url, '/public/index.html');
  assert.equal(response.json.headers['x-auth-sub'], undefined);
});

test('answers a protected route without a token with 401', async () => {
  const response = await call(gateway.baseUrl, '/orders/42');

  assert.equal(response.status, 401);
  assert.equal(response.json.code, 'missing_token');
});

test('forwards an authorized request with the identity headers', async () => {
  const token = await signToken({ sub: 'user-42', roles: ['operator'] });

  const response = await call(gateway.baseUrl, '/orders/42', {
    headers: { authorization: `Bearer ${token}`, 'x-request-id': 'req-1' },
  });

  assert.equal(response.status, 200);
  assert.equal(response.json.url, '/orders/42');
  assert.equal(response.json.headers['x-auth-sub'], 'user-42');
  assert.equal(response.json.headers['x-auth-roles'], 'operator');
  assert.equal(response.json.headers['x-request-id'], 'req-1');
});

test('answers a route with 403 when a namespace scope is absent', async () => {
  const token = await signToken({ scope: ['catalog:read'] });
  const before = gateway.requests.length;

  const response = await call(gateway.baseUrl, '/admin/export', {
    headers: { authorization: `Bearer ${token}` },
  });

  assert.equal(response.status, 403);
  assert.equal(response.json.code, 'insufficient_scope');
  assert.deepEqual(response.json.missing_scopes, ['admin:export']);
  assert.equal(gateway.requests.length, before);
});
