'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { compileSchema } = require('../src/schema/compile');
const { coerceScalar } = require('../src/validate/coerce');

test('numeric strings coerce to integer plan nodes', async () => {
  const plan = await compileSchema({
    type: 'object',
    properties: { count: { type: 'integer' } },
  });
  assert.equal(coerceScalar('12', plan.properties.count.type), 12);
});

test('boolean plan nodes coerce the lowercase literal', async () => {
  const plan = await compileSchema({
    type: 'object',
    properties: { enabled: { type: 'boolean' } },
  });
  assert.equal(coerceScalar('true', plan.properties.enabled.type), true);
  assert.equal(coerceScalar('false', plan.properties.enabled.type), false);
});

test('non-string values are returned untouched', async () => {
  const plan = await compileSchema({ type: 'integer' });
  assert.equal(coerceScalar(5, plan.type), 5);
  assert.equal(coerceScalar(true, plan.type), true);
  assert.equal(coerceScalar(null, plan.type), null);
});

test('a string type leaves strings alone', async () => {
  const plan = await compileSchema({ type: 'string' });
  assert.equal(coerceScalar('12', plan.type), '12');
});