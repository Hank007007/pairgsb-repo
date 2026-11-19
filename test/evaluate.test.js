import test from 'node:test';
import assert from 'node:assert/strict';

import { decide, evaluate, principalKeys } from '../src/acl/evaluate.js';

function ruleset(entries) {
  return { principals: entries };
}

test('principalKeys maps identity fields to prefixed keys', () => {
  assert.deepEqual(
    principalKeys({ cn: 'gateway', username: 'alice', clientId: 'dev-1' }),
    ['cn:gateway', 'user:alice', 'client:dev-1'],
  );
  assert.deepEqual(principalKeys({ clientId: 'dev-1' }), ['client:dev-1']);
  assert.deepEqual(principalKeys({}), []);
});

test('decide allows when the single rule matches the subject', () => {
  const result = decide(
    [{ action: 'publish', topic: 'sensors/a/state', effect: 'allow' }],
    'publish',
    'sensors/a/state',
  );
  assert.equal(result.effect, 'allow');
  assert.equal(result.rule.effect, 'allow');
});

test('decide denies when the single rule does not match', () => {
  const result = decide(
    [{ action: 'publish', topic: 'sensors/a/state', effect: 'allow' }],
    'publish',
    'farm/a/state',
  );
  assert.equal(result.effect, 'deny');
  assert.equal(result.rule, null);
});

test('decide ignores a rule written for another action', () => {
  const result = decide(
    [{ action: 'subscribe', topic: 'sensors/a/state', effect: 'allow' }],
    'publish',
    'sensors/a/state',
  );
  assert.equal(result.effect, 'deny');
});

test('evaluate allows through the username principal', () => {
  const result = evaluate({
    rules: ruleset({
      'user:alice': {
        rules: [{ action: 'publish', topic: 'sensors/a/state', effect: 'allow' }],
      },
    }),
    identity: { username: 'alice' },
    action: 'publish',
    subject: 'sensors/a/state',
  });

  assert.equal(result.allowed, true);
  assert.equal(result.reason, 'matched-allow');
  assert.equal(result.principal, 'user:alice');
});

test('evaluate defaults to deny for an unknown principal', () => {
  const result = evaluate({
    rules: ruleset({}),
    identity: { username: 'nobody' },
    action: 'publish',
    subject: 'sensors/a/state',
  });

  assert.equal(result.allowed, false);
  assert.equal(result.reason, 'default-deny');
});

test('evaluate reports a deny rule that matches', () => {
  const result = evaluate({
    rules: ruleset({
      'user:alice': {
        rules: [{ action: 'subscribe', topic: 'farm/private', effect: 'deny' }],
      },
    }),
    identity: { username: 'alice' },
    action: 'subscribe',
    subject: 'farm/private',
  });

  assert.equal(result.allowed, false);
  assert.equal(result.reason, 'matched-deny');
  assert.equal(result.rule.effect, 'deny');
});
