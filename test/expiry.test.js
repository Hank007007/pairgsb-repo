'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { buildService } = require('./helpers');

test('verify: a freshly minted grant is accepted', () => {
  const { service } = buildService();
  const { token } = service.issue({
    tenantId: 'tenant-a',
    path: '/tenant-a/docs',
    actions: ['read'],
    ttlMs: 30_000,
  });

  const result = service.verify(token, {
    action: 'read',
    requestPath: '/tenant-a/docs/q3.txt',
  });

  assert.equal(result.ok, true);
});

test('verify: the grant payload carries the expiry it was minted with', () => {
  const { service, clock } = buildService();
  clock.value = Date.UTC(2024, 8, 3, 14, 2, 0);
  const { grant } = service.issue({
    tenantId: 'tenant-a',
    path: '/tenant-a/docs',
    actions: ['read'],
    ttlMs: 30_000,
  });

  assert.equal(grant.issuedAtMs, clock.value);
  assert.equal(grant.expiresAtMs, grant.issuedAtMs + grant.ttlMs);
  assert.ok(grant.expiresAtMs > grant.issuedAtMs);
});
