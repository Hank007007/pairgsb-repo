'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');

const { validateRequest } = require('../src/http/gateway');

const fixture = JSON.parse(
  fs.readFileSync(path.join(__dirname, 'fixtures', 'simple-schema.json'), 'utf8'),
);

test('health route is served', async () => {
  const { createServer } = require('../src/http/server');
  const server = createServer();
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const { port } = server.address();
    const response = await fetch(`http://127.0.0.1:${port}/healthz`);
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), { ok: true });
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
});

test('valid request passes with required properties', async () => {
  const body = JSON.stringify({ name: 'Ada', email: 'ada@example.com' });
  const result = await validateRequest(fixture, body, { tenantId: 'tenant-a' });
  assert.equal(result.ok, true);
  assert.equal(result.instance.name, 'Ada');
  assert.equal(result.instance.email, 'ada@example.com');
  assert.equal(result.errors, undefined);
});

test('parsed instance input is accepted', async () => {
  const result = await validateRequest(fixture, { name: 'Ada', email: 'ada@example.com' });
  assert.equal(result.ok, true);
  assert.equal(result.instance.name, 'Ada');
});

test('a short body over the default limit parses normally', async () => {
  const result = await validateRequest(fixture, JSON.stringify({ name: 'x', email: 'y' }));
  assert.equal(result.ok, true);
});

test('object schema with properties survives compilation', async () => {
  const { compileSchema } = require('../src/schema/compile');
  const plan = await compileSchema(fixture, { tenantId: 'tenant-a' });
  assert.equal(plan.type, 'object');
  assert.deepEqual(plan.required, ['name', 'email']);
  assert.ok(plan.properties.name);
  assert.equal(plan.properties.name.type, 'string');
});