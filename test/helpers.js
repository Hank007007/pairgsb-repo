'use strict';

const { createGrantService } = require('../src/index');
const { createPolicyRegistry } = require('../src/policy/tenant');
const { createRevocationStore } = require('../src/store/revoke');

const TENANT_A = {
  tenantId: 'tenant-a',
  root: '/tenant-a',
  adminActions: ['read', 'write'],
  defaultTtlMs: 60_000,
  maxTtlMs: 600_000,
};

const TENANT_B = {
  tenantId: 'tenant-b',
  root: '/tenant-b',
  adminActions: ['read'],
  defaultTtlMs: 60_000,
  maxTtlMs: 600_000,
};

const TENANT_FAST = {
  tenantId: 'tenant-fast',
  root: '/tenant-fast',
  adminActions: ['read'],
  defaultTtlMs: 250,
  maxTtlMs: 250,
};

/**
 * Build a service backed by an in-memory policy table and a controllable
 * clock. `clock.value` may be reassigned between calls.
 */
function buildService(overrides = {}) {
  const policies = createPolicyRegistry([TENANT_A, TENANT_B, TENANT_FAST]);
  const clock = { value: Date.now() };
  const store = overrides.store ?? createRevocationStore();
  const service = createGrantService({
    policies,
    store,
    now: () => clock.value,
  });
  return { service, clock, policies, store };
}

module.exports = {
  TENANT_A,
  TENANT_B,
  TENANT_FAST,
  buildService,
};
