import assert from 'node:assert/strict';
import test from 'node:test';

import { matchGlob, PolicyEngine, normalizeRules, PolicyFormatError } from '../src/index.js';

function request(resource, extra = {}) {
  return {
    principal: '/users/alice',
    resource,
    action: 'read',
    ...extra,
  };
}

test('no matching rule and an empty rule set both deny by default', () => {
  const empty = new PolicyEngine({ rules: [] });
  assert.deepEqual(empty.decide(request('/healthz')), {
    allow: false,
    matchedPolicy: null,
    reason: 'no matching policy',
  });

  const engine = new PolicyEngine({
    rules: [
      {
        id: 'docs-read',
        principal: '/users/*',
        resource: '/docs/**',
        action: ['read'],
        effect: 'permit',
      },
    ],
  });
  assert.deepEqual(engine.decide(request('/healthz')), {
    allow: false,
    matchedPolicy: null,
    reason: 'no matching policy',
  });
});

test('star matches one segment and globstar matches zero or more segments', () => {
  assert.equal(matchGlob('/docs/*', '/docs/a'), true);
  assert.equal(matchGlob('/docs/*', '/docs'), false);
  assert.equal(matchGlob('/docs/*', '/docs/a/b'), false);

  assert.equal(matchGlob('/docs/**', '/docs'), true);
  assert.equal(matchGlob('/docs/**', '/docs/a'), true);
  assert.equal(matchGlob('/docs/**', '/docs/a/b'), true);
  assert.equal(matchGlob('/docs/**', '/docsx'), false);
});

test('resource candidates walk from the requested path up to its ancestors', () => {
  const engine = new PolicyEngine({
    rules: [
      {
        id: 'invoices-read',
        principal: '/users/*',
        resource: '/invoices',
        action: ['read'],
        effect: 'permit',
      },
    ],
  });

  assert.equal(engine.decide(request('/invoices')).matchedPolicy, 'invoices-read');
  assert.equal(engine.decide(request('/invoices/123')).matchedPolicy, 'invoices-read');
  assert.equal(engine.decide(request('/invoices/123/items')).matchedPolicy, 'invoices-read');
});

test('a matching deny wins regardless of declaration order', () => {
  const permit = {
    id: 'oncall-read',
    principal: '/users/*',
    resource: '/tickets/**',
    action: ['read'],
    condition: { oncall: true },
    effect: 'permit',
  };
  const deny = {
    id: 'archived-deny',
    principal: '/users/*',
    resource: '/tickets/**',
    action: ['read'],
    condition: { archived: true },
    effect: 'deny',
  };
  const context = { oncall: true, archived: true };

  for (const rules of [[permit, deny], [deny, permit]]) {
    const engine = new PolicyEngine({ rules });
    assert.deepEqual(engine.decide(request('/tickets/42', { context })), {
      allow: false,
      matchedPolicy: 'archived-deny',
      reason: 'matched deny rule archived-deny',
    });
  }
});

test('a condition that references a missing context key skips the rule', () => {
  const engine = new PolicyEngine({
    rules: [
      {
        id: 'clearance-required',
        principal: '/users/*',
        resource: '/mobile/**',
        action: ['read'],
        condition: { clearance: { gte: 3 } },
        effect: 'permit',
      },
      {
        id: 'mobile-read',
        principal: '/users/*',
        resource: '/mobile/**',
        action: ['read'],
        effect: 'permit',
      },
    ],
  });

  const result = engine.decide(request('/mobile/home'));
  assert.equal(result.allow, true);
  assert.equal(result.matchedPolicy, 'mobile-read');
});

test('numeric strings and numbers use numeric comparison', () => {
  const engine = new PolicyEngine({
    rules: [
      {
        id: 'rate-limit',
        principal: '/users/*',
        resource: '/risk/check',
        action: ['read'],
        condition: { attempts: { gt: 9 } },
        effect: 'deny',
      },
    ],
  });

  assert.equal(engine.decide(request('/risk/check', { context: { attempts: 10 } })).matchedPolicy, 'rate-limit');
  assert.equal(engine.decide(request('/risk/check', { context: { attempts: 9 } })).matchedPolicy, null);
  assert.equal(engine.decide(request('/risk/check', { context: { attempts: '10' } })).matchedPolicy, 'rate-limit');
});

test('decideMany cache keys include resource and context', () => {
  const rules = [
    {
      id: 'reports-read',
      principal: '/users/*',
      resource: '/reports/**',
      action: ['read'],
      condition: { role: 'member' },
      effect: 'permit',
    },
    {
      id: 'reports-restricted',
      principal: '/users/*',
      resource: '/reports/finance',
      action: ['read'],
      condition: { role: 'member' },
      effect: 'deny',
    },
  ];
  const engine = new PolicyEngine({ rules });
  const requests = [
    request('/reports/a', { context: { role: 'member' } }),
    request('/reports/b', { context: { role: 'member' } }),
    request('/reports/finance', { context: { role: 'member' } }),
    request('/reports/a', { context: { role: 'guest' } }),
  ];

  assert.deepEqual(
    engine.decideMany(requests),
    requests.map((entry) => engine.decide(entry)),
  );
});

test('normalizeRules accepts the rules wrapper and names the rule on field errors', () => {
  const rule = {
    id: 'missing-principal',
    resource: '/docs/**',
    action: ['read'],
    effect: 'permit',
  };

  assert.equal(normalizeRules({ rules: [{ ...rule, principal: '/users/*' }] }).length, 1);
  assert.throws(
    () => normalizeRules({ rules: [rule] }, { source: 'documents.json' }),
    (error) =>
      error instanceof PolicyFormatError &&
      error.message.includes('documents.json') &&
      error.message.includes('missing-principal'),
  );
});
