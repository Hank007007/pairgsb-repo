'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { buildService } = require('./helpers');

function mint(service, overrides = {}) {
  return service.issue({
    tenantId: 'tenant-a',
    path: '/tenant-a/docs',
    actions: ['read'],
    ttlMs: 30_000,
    ...overrides,
  });
}

test('verify: accepts a fresh grant for the path it covers', () => {
  const { service } = buildService();
  const { token, grant } = mint(service);

  const result = service.verify(token, {
    action: 'read',
    requestPath: '/tenant-a/docs/q3.txt',
  });

  assert.equal(result.ok, true);
  assert.deepEqual(result.grant, grant);
});

test('verify: accepts the granted path itself', () => {
  const { service } = buildService();
  const { token } = mint(service);

  const result = service.verify(token, {
    action: 'read',
    requestPath: '/tenant-a/docs',
  });

  assert.equal(result.ok, true);
});

test('verify: rejects an action the grant does not cover', () => {
  const { service } = buildService();
  const { token } = mint(service);

  const result = service.verify(token, {
    action: 'write',
    requestPath: '/tenant-a/docs/q3.txt',
  });

  assert.equal(result.ok, false);
  assert.equal(result.reason, 'action-not-granted');
});

test('verify: rejects a malformed token without throwing', () => {
  const { service } = buildService();

  const result = service.verify('garbage', {
    action: 'read',
    requestPath: '/tenant-a/docs/q3.txt',
  });

  assert.equal(result.ok, false);
  assert.equal(result.reason, 'malformed');
});

test('verify: rejects a request without an action', () => {
  const { service } = buildService();
  const { token } = mint(service);

  const result = service.verify(token, { requestPath: '/tenant-a/docs/q3.txt' });

  assert.equal(result.ok, false);
  assert.equal(result.reason, 'bad-request');
});

test('verify: a grant minted with both actions serves either of them', () => {
  const { service } = buildService();
  const { token } = mint(service, { actions: ['read', 'write'] });

  for (const action of ['read', 'write']) {
    const result = service.verify(token, {
      action,
      requestPath: '/tenant-a/docs/q3.txt',
    });
    assert.equal(result.ok, true, `expected ${action} to be accepted`);
  }
});
