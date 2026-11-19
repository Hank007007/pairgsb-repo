'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const policy = require('../lib/policy');
const { makeCaKey, makeTmpDir, removeDir } = require('../test-support/keys');

test('every role exposes a non-empty principal set and a positive default ttl', () => {
  for (const role of policy.ROLE_NAMES) {
    const entry = policy.getRole(role);
    assert.ok(entry.principals.length > 0, `${role} has no principals`);
    assert.ok(entry.ttlSeconds > 0, `${role} has no default ttl`);
    assert.ok(entry.ttlSeconds <= policy.MAX_CERT_TTL_SECONDS);
  }
});

test('deploy may only log in as deploy', () => {
  assert.deepEqual(policy.allowedPrincipals('deploy'), ['deploy']);
  assert.equal(policy.defaultTtlSeconds('deploy'), 3600);
});

test('sre and ci map to the documented principal sets', () => {
  assert.deepEqual(policy.allowedPrincipals('sre'), ['sre', 'ops']);
  assert.deepEqual(policy.allowedPrincipals('ci'), ['ci', 'deploy']);
  assert.deepEqual(policy.allowedPrincipals('breakglass'), ['root']);
});

test('allowedPrincipals returns a copy', () => {
  const first = policy.allowedPrincipals('deploy');
  first.push('root');
  assert.deepEqual(policy.allowedPrincipals('deploy'), ['deploy']);
});

test('unknown roles are rejected', () => {
  assert.throws(() => policy.getRole('admin'), (error) => {
    assert.equal(error.code, 'invalid_role');
    return true;
  });
  assert.throws(() => policy.getRole(''), /missing a role/);
});

test('the ttl ceiling is eight hours', () => {
  assert.equal(policy.MAX_CERT_TTL_SECONDS, 28800);
});

test('CA key files are enumerated by suffix', () => {
  const ca = require('../lib/ca');
  const dir = makeTmpDir('ca-listing');
  try {
    makeCaKey(dir, 'certbrace-v2');
    makeCaKey(dir, 'certbrace-v1.retired');
    fs.writeFileSync(path.join(dir, 'notes.txt'), 'not a key\n');

    const files = ca.listCaKeyFiles(dir);
    assert.deepEqual(files.slice().sort(), ['certbrace-v1.retired.key', 'certbrace-v2.key']);
  } finally {
    removeDir(dir);
  }
});

test('key ids and retirement markers are read from file names', () => {
  const ca = require('../lib/ca');
  assert.equal(ca.keyIdFromFile('certbrace-v2.key'), 'certbrace-v2');
  assert.equal(ca.isRetired('certbrace-v1.retired'), true);
  assert.equal(ca.isRetired('certbrace-v2'), false);
});

test('an empty CA directory yields no usable key', () => {
  const ca = require('../lib/ca');
  const dir = makeTmpDir('ca-empty');
  try {
    assert.deepEqual(ca.listCaKeyFiles(dir), []);
  } finally {
    removeDir(dir);
  }
});
