import test from 'node:test';
import assert from 'node:assert/strict';

import { SchemaRegistry } from '../lib/schema/registry.js';
import { cacheKey, resolveType } from '../lib/schema/resolve.js';
import { loadSnapshot } from '../tools/fixtures.js';

test('cacheKey combines type and version', () => {
  assert.equal(cacheKey('Telemetry', 1), 'Telemetry#1');
  assert.equal(cacheKey('Telemetry', 2), 'Telemetry#2');
  assert.notEqual(cacheKey('Telemetry', 1), cacheKey('Telemetry', 2));
});

test('cacheKey keeps distinct types apart at the same version', () => {
  assert.notEqual(cacheKey('Telemetry', 1), cacheKey('SensorReading', 1));
});

test('resolveType exposes the declared fields of a type', () => {
  const reg = new SchemaRegistry(loadSnapshot('telemetry.v1.json'));
  const schema = resolveType(reg, 'Telemetry');

  assert.equal(schema.type, 'Telemetry');
  assert.equal(schema.version, 1);
  assert.deepEqual(
    schema.fields.map((field) => field.number),
    [1, 2, 3, 4, 5, 6],
  );
});

test('fieldByNumber finds a field by its wire number', () => {
  const reg = new SchemaRegistry(loadSnapshot('telemetry.v1.json'));
  const schema = resolveType(reg, 'Telemetry');

  assert.equal(schema.fieldByNumber(1).name, 'device_id');
  assert.equal(schema.fieldByNumber(4).messageType, 'SensorReading');
  assert.equal(schema.fieldByNumber(999), undefined);
});

test('a nested type can be resolved from a schema', () => {
  const reg = new SchemaRegistry(loadSnapshot('telemetry.v1.json'));
  const schema = resolveType(reg, 'Telemetry');
  const nested = schema.getType('SensorReading');

  assert.equal(nested.type, 'SensorReading');
  assert.equal(nested.fieldByNumber(2).name, 'millivolts');
});
