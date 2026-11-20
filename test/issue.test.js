'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { buildService, TENANT_A } = require('./helpers');
const { decodeGrant } = require('../src/codec');

test('issue: signs a grant for a clean path inside the tenant root', () => {
  const { service, clock } = buildService();
  clock.value = Date.UTC(2024, 8, 3, 14, 2, 0);
  const { token, grant } = service.issue({
    tenantId: 'tenant-a',
    path: '/tenant-a/docs/q3.txt',
    actions: ['read'],
    ttlMs: 30_000,
  });

  assert.equal(typeof token, 'string');
  assert.equal(grant.tenantId, 'tenant-a');
  assert.equal(grant.path, '/tenant-a/docs/q3.txt');
  assert.deepEqual(grant.actions, ['read']);
  assert.equal(grant.issuedAtMs, clock.value);
  assert.equal(grant.expiresAtMs, grant.issuedAtMs + grant.ttlMs);
});

test('issue: the token carries the same payload the caller received', () => {
  const { service } = buildService();
  const { token, grant } = service.issue({
    tenantId: 'tenant-a',
    path: '/tenant-a/docs/q3.txt',
    actions: ['read'],
    ttlMs: 30_000,
  });

  assert.deepEqual(decodeGrant(token), grant);
});

test('issue: an omitted ttlMs falls back to the tenant default', () => {
  const { service } = buildService();
  const { grant } = service.issue({
    tenantId: 'tenant-a',
    path: '/tenant-a/docs',
    actions: ['read'],
  });

  assert.equal(grant.ttlMs, TENANT_A.defaultTtlMs);
});

test('issue: requested actions equal to the admin actions are recorded', () => {
  const { service } = buildService();
  const { grant } = service.issue({
    tenantId: 'tenant-a',
    path: '/tenant-a/docs',
    actions: ['read', 'write'],
  });

  assert.deepEqual(grant.actions.slice().sort(), ['read', 'write']);
});

test('issue: expiresAtMs is derived from issuedAtMs and the granted ttlMs', () => {
  const { service } = buildService();
  const { grant } = service.issue({
    tenantId: 'tenant-fast',
    path: '/tenant-fast/objects',
    actions: ['read'],
    ttlMs: 100,
  });

  assert.equal(grant.expiresAtMs - grant.issuedAtMs, grant.ttlMs);
  assert.ok(grant.ttlMs > 0);
});

test('issue: a clean relative path is accepted and echoed into the token', () => {
  const { service } = buildService();
  const { token, grant } = service.issue({
    tenantId: 'tenant-a',
    path: 'docs/q3.txt',
    actions: ['read'],
  });

  assert.deepEqual(decodeGrant(token), grant);
});

test('issue: rejects a request without actions', () => {
  const { service } = buildService();
  assert.throws(
    () => service.issue({ tenantId: 'tenant-a', path: '/tenant-a/docs', actions: [] }),
    TypeError,
  );
});

test('issue: rejects a non-positive ttlMs', () => {
  const { service } = buildService();
  assert.throws(
    () =>
      service.issue({
        tenantId: 'tenant-a',
        path: '/tenant-a/docs',
        actions: ['read'],
        ttlMs: 0,
      }),
    TypeError,
  );
});

test('issue: rejects an empty tenantId', () => {
  const { service } = buildService();
  assert.throws(
    () => service.issue({ tenantId: '', path: '/tenant-a/docs', actions: ['read'] }),
    TypeError,
  );
});
