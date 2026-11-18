import assert from 'node:assert/strict';
import test from 'node:test';

import { decideMany } from '../src/batch.js';
import { TtlCache } from '../src/cache.js';
import { PolicyEngine } from '../src/engine.js';

const RULES = [
  {
    id: 'health-read',
    principal: '/services/gateway',
    resource: '/healthz',
    action: ['read'],
    effect: 'permit',
  },
];

function healthRequest() {
  return {
    principal: '/services/gateway',
    resource: '/healthz',
    action: 'read',
  };
}

test('a batch returns one result per request, in input order', () => {
  const engine = new PolicyEngine({ rules: RULES });
  const results = decideMany(engine, [healthRequest(), healthRequest(), healthRequest()], null);

  assert.equal(results.length, 3);
  for (const result of results) {
    assert.equal(result.allow, true);
    assert.equal(result.matchedPolicy, 'health-read');
  }

  assert.deepEqual(decideMany(engine, [], null), []);
  assert.throws(() => decideMany(engine, healthRequest(), null), TypeError);
});

test('every batch entry equals a single decision for the same request', () => {
  const engine = new PolicyEngine({ rules: RULES, cache: false });
  const request = healthRequest();
  assert.deepEqual(engine.decideMany([request])[0], engine.decide(request));
});

test('a cache hit hands back a copy, not the stored decision', () => {
  const cache = new TtlCache({ ttlMs: 1_000 });
  const engine = new PolicyEngine({ rules: RULES, cache });

  const [first] = decideMany(engine, [healthRequest()], cache);
  first.allow = false;

  const [second] = decideMany(engine, [healthRequest()], cache);
  assert.equal(second.allow, true);
});
