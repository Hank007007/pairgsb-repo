'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const policy = require('../lib/policy');
const { CertbraceError } = require('../lib/errors');

test('a request without a role is rejected', () => {
  assert.throws(
    () => policy.getRole(undefined),
    (error) => error instanceof CertbraceError && error.status === 400,
  );
});

test('a role name is matched exactly', () => {
  assert.throws(() => policy.getRole('Deploy'), /unknown role: Deploy/);
  assert.throws(() => policy.getRole('deploy '), /unknown role/);
});

test('the policy table is frozen', () => {
  assert.equal(Object.isFrozen(policy.ROLES), true);
  assert.equal(Object.isFrozen(policy.ROLES.deploy), true);
  assert.throws(() => {
    policy.ROLES.deploy.ttlSeconds = 999999;
  }, TypeError);
});
