'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { validateRequest, planCache } = require('../src/http/gateway');
const { compileSchema } = require('../src/schema/compile');

const schemaA = { type: 'object', properties: { a: { type: 'string' } } };
const schemaB = { type: 'object', properties: { b: { type: 'string' } } };

test('a compiled plan records the tenant that compiled it', async () => {
  const plan = await compileSchema(schemaA, { tenantId: 'tenant-a' });
  assert.equal(plan.tenantId, 'tenant-a');
});

test('compileSchema honours the tenantId option', async () => {
  const plan = await compileSchema(schemaB, { tenantId: 'tenant-b' });
  assert.equal(plan.tenantId, 'tenant-b');
});

test('the same schema and tenant are cached across requests', async () => {
  planCache.clear();
  const first = await validateRequest(schemaA, { a: 'x' }, { tenantId: 'tenant-a' });
  const second = await validateRequest(schemaA, { a: 'y' }, { tenantId: 'tenant-a' });
  assert.equal(first.ok, true);
  assert.equal(second.ok, true);
  assert.equal(planCache.size, 1);
});

test('a schema with a format keyword compiles and validates', async () => {
  const result = await validateRequest(
    { type: 'object', properties: { ref: { type: 'string', format: 'uuid' } } },
    { ref: '6f1c0f9e-2b7a-4c1d-9e8f-1a2b3c4d5e6f' },
    { tenantId: 'tenant-a' },
  );
  assert.equal(result.ok, true);
});