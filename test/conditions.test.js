import assert from 'node:assert/strict';
import test from 'node:test';

import { evaluate } from '../src/conditions.js';

test('an omitted or empty condition always holds', () => {
  assert.equal(evaluate(undefined, {}), true);
  assert.equal(evaluate(null, {}), true);
  assert.equal(evaluate({}, {}), true);
});

test('shorthand values compare for equality', () => {
  assert.equal(evaluate({ role: 'member' }, { role: 'member' }), true);
  assert.equal(evaluate({ role: 'member' }, { role: 'admin' }), false);
  assert.equal(evaluate({ authenticated: true }, { authenticated: true }), true);
  assert.equal(evaluate({ authenticated: true }, { authenticated: false }), false);
});

test('the in operator checks membership of a list', () => {
  const condition = { role: { in: ['member', 'admin'] } };
  assert.equal(evaluate(condition, { role: 'member' }), true);
  assert.equal(evaluate(condition, { role: 'guest' }), false);
});

test('numeric comparisons order single digit values', () => {
  assert.equal(evaluate({ attempts: { gt: 4 } }, { attempts: 5 }), true);
  assert.equal(evaluate({ attempts: { gt: 4 } }, { attempts: 3 }), false);
  assert.equal(evaluate({ attempts: { lt: 5 } }, { attempts: 3 }), true);
  assert.equal(evaluate({ score: { lte: 3 } }, { score: 3 }), true);
  assert.equal(evaluate({ score: { gte: 7 } }, { score: 7 }), true);
  assert.equal(evaluate({ score: { gte: 7 } }, { score: 6 }), false);
});

test('the ne operator is the negation of equality', () => {
  assert.equal(evaluate({ status: { ne: 'closed' } }, { status: 'open' }), true);
  assert.equal(evaluate({ status: { ne: 'closed' } }, { status: 'closed' }), false);
});

test('multiple operators and keys must all hold', () => {
  assert.equal(evaluate({ score: { gte: 2, lte: 6 } }, { score: 4 }), true);
  assert.equal(evaluate({ score: { gte: 2, lte: 6 } }, { score: 8 }), false);
  assert.equal(
    evaluate({ role: 'member', archived: false }, { role: 'member', archived: false }),
    true,
  );
  assert.equal(
    evaluate({ role: 'member', archived: false }, { role: 'member', archived: true }),
    false,
  );
});
