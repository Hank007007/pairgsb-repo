'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { validateRequest } = require('../src/http/gateway');

test('a scalar string field validates without extra noise', async () => {
  const schema = {
    type: 'object',
    properties: { name: { type: 'string' } },
    required: ['name'],
  };
  const result = await validateRequest(schema, { name: 'Ada' }, { tenantId: 'tenant-a' });
  assert.equal(result.ok, true);
});

test('a plain pattern accepts a matching value', async () => {
  const schema = {
    type: 'object',
    properties: { code: { type: 'string', pattern: '^[A-Z]{3}$' } },
  };
  const result = await validateRequest(schema, { code: 'ABC' }, { tenantId: 'tenant-a' });
  assert.equal(result.ok, true);
});

test('a declared default is filled in for a missing property', async () => {
  const schema = {
    type: 'object',
    properties: { region: { type: 'string', default: 'eu-west' } },
  };
  const result = await validateRequest(schema, {}, { tenantId: 'tenant-a' });
  assert.equal(result.ok, true);
  assert.equal(result.instance.region, 'eu-west');
});

test('an integer string is normalised for a numeric field', async () => {
  const schema = {
    type: 'object',
    properties: { count: { type: 'integer' } },
  };
  const result = await validateRequest(schema, { count: '7' }, { tenantId: 'tenant-a' });
  assert.equal(result.ok, true);
  assert.equal(result.instance.count, 7);
});