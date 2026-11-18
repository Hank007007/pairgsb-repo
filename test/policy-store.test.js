import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

import { createEngine } from '../src/index.js';
import { loadPolicies, parsePolicyDocument } from '../src/policy-store.js';
import { PolicyFormatError } from '../src/rules.js';

const FIXTURE_DIR = fileURLToPath(new URL('./fixtures/policies', import.meta.url));

const RULE = {
  id: 'fixture-health-read',
  principal: '/services/*',
  resource: '/healthz',
  action: ['read'],
  effect: 'permit',
};

test('parsePolicyDocument accepts a bare array or a rules wrapper', () => {
  assert.equal(parsePolicyDocument([RULE]).length, 1);
  assert.equal(parsePolicyDocument({ rules: [RULE] }).length, 1);
  assert.throws(() => parsePolicyDocument({ nope: true }), PolicyFormatError);
});

test('loadPolicies reads every json file, in file name order', async () => {
  const rules = await loadPolicies(FIXTURE_DIR);
  assert.deepEqual(
    rules.map((rule) => rule.id),
    ['fixture-health-read', 'fixture-release-read', 'fixture-release-locked'],
  );
});

test('createEngine builds a usable engine from a policy directory', async () => {
  const engine = await createEngine({ policyDir: FIXTURE_DIR });
  const result = engine.decide({
    principal: '/services/gateway',
    resource: '/healthz',
    action: 'read',
  });

  assert.equal(result.allow, true);
  assert.equal(result.matchedPolicy, 'fixture-health-read');
});

test('a missing policy directory is reported as a format error', async () => {
  await assert.rejects(
    () => loadPolicies(fileURLToPath(new URL('./fixtures/absent', import.meta.url))),
    PolicyFormatError,
  );
});
