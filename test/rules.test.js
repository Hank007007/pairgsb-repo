import assert from 'node:assert/strict';
import test from 'node:test';

import { PolicyFormatError, normalizeRule, normalizeRules } from '../src/rules.js';

const VALID = {
  id: 'health-read',
  principal: '/services/*',
  resource: '/healthz',
  action: ['read'],
  effect: 'permit',
};

test('normalizeRule freezes a validated rule and defaults its description', () => {
  const rule = normalizeRule(VALID, { source: 'test' });

  assert.equal(rule.id, 'health-read');
  assert.equal(rule.description, '');
  assert.ok(Object.isFrozen(rule));
  assert.ok(Object.isFrozen(rule.action));
});

test('normalizeRule rejects structurally invalid rules', () => {
  assert.throws(() => normalizeRule({ ...VALID, action: [] }, { source: 'test' }), PolicyFormatError);
  assert.throws(() => normalizeRule({ ...VALID, effect: 'allow' }, { source: 'test' }), PolicyFormatError);
  assert.throws(
    () => normalizeRule({ ...VALID, condition: { role: { matches: 'x' } } }, { source: 'test' }),
    PolicyFormatError,
  );
});

test('normalizeRules refuses duplicate ids and names the offending rule', () => {
  assert.throws(() => normalizeRules([VALID, { ...VALID }], { source: 'test' }), /health-read/);
});
