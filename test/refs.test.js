'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { validateRequest, readBody } = require('../src/http/gateway');
const { resolvePointer, pointerTokens } = require('../src/schema/refs');

test('a local pointer resolves a nested definition', () => {
  const doc = { $defs: { name: { type: 'string' } } };
  assert.deepEqual(resolvePointer(doc, '/$defs/name'), { type: 'string' });
});

test('pointer tokens keep the raw split for a plain fragment', () => {
  assert.deepEqual(pointerTokens('/a/b'), ['a', 'b']);
});

test('a body given as a raw string is parsed as JSON', async () => {
  const raw = JSON.stringify({ a: 'x' });
  assert.deepEqual(await readBody(raw), raw);
});

test('an async iterable body is concatenated', async () => {
  async function* chunks() {
    yield '{"a":';
    yield '"x"}';
  }
  assert.equal(await readBody(chunks()), '{"a":"x"}');
});

test('a schema with a plain object type validates a matching instance', async () => {
  const result = await validateRequest(
    { type: 'object', properties: { a: { type: 'string' } } },
    { a: 'x' },
    { tenantId: 'tenant-a' },
  );
  assert.equal(result.ok, true);
});