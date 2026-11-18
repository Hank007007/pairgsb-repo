import assert from 'node:assert/strict';
import test from 'node:test';

import { PolicyEngine } from '../src/engine.js';

const HEALTH_READ = {
  id: 'health-read',
  principal: '/services/gateway',
  resource: '/healthz',
  action: ['read'],
  effect: 'permit',
};

function healthRequest() {
  return {
    principal: '/services/gateway',
    resource: '/healthz',
    action: 'read',
  };
}

test('a permit rule grants a matching request and reports its fields', () => {
  const engine = new PolicyEngine({ rules: [HEALTH_READ] });
  const result = engine.decide(healthRequest());

  assert.equal(result.allow, true);
  assert.equal(result.matchedPolicy, 'health-read');
  assert.match(result.reason, /health-read/);
  assert.deepEqual(Object.keys(result).sort(), ['allow', 'matchedPolicy', 'reason']);
});

test('a deny rule that matches names itself', () => {
  const engine = new PolicyEngine({
    rules: [
      {
        id: 'release-locked',
        principal: '/users/alice',
        resource: '/release',
        action: ['write'],
        condition: { locked: true },
        effect: 'deny',
      },
    ],
  });

  const result = engine.decide({
    principal: '/users/alice',
    resource: '/release',
    action: 'write',
    context: { locked: true },
  });

  assert.equal(result.allow, false);
  assert.equal(result.matchedPolicy, 'release-locked');
});

test('a rule whose condition does not hold is passed over', () => {
  const engine = new PolicyEngine({
    rules: [
      {
        id: 'release-locked',
        principal: '/users/alice',
        resource: '/release',
        action: ['write'],
        condition: { locked: true },
        effect: 'deny',
      },
      {
        id: 'release-write',
        principal: '/users/alice',
        resource: '/release',
        action: ['write'],
        condition: { locked: false },
        effect: 'permit',
      },
    ],
  });

  const result = engine.decide({
    principal: '/users/alice',
    resource: '/release',
    action: 'write',
    context: { locked: false },
  });

  assert.equal(result.allow, true);
  assert.equal(result.matchedPolicy, 'release-write');
});

test('a request without the required fields is rejected', () => {
  const engine = new PolicyEngine({ rules: [HEALTH_READ] });
  assert.throws(() => engine.decide({ resource: '/healthz', action: 'read' }), TypeError);
  assert.throws(() => engine.decide({ principal: '/services/gateway', action: 'read' }), TypeError);
});
