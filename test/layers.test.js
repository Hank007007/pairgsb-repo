import test from 'node:test';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

import { load, envOverlay, deepMerge } from '../src/index.js';

const here = dirname(fileURLToPath(import.meta.url));
const BASE = resolve(here, 'fixtures/base.json');
const STAGING = resolve(here, 'fixtures/staging.json');

test('the first layer is always treated as the base', () => {
  const { config } = load({
    defaults: {},
    layers: [BASE, STAGING],
    env: {},
    argv: [],
  });
  assert.equal(config.logging.level, 'trace');
  assert.equal(config.db.pool.size, 16);
});

test('a later overlay does not erase keys it does not mention', () => {
  const { config } = load({
    defaults: {},
    layers: [BASE, STAGING],
    env: {},
    argv: [],
  });
  assert.equal(config.db.host, 'db.internal');
  assert.equal(config.db.protocol === undefined, true);
  assert.equal(config.service.name, 'orders');
});

test('missing layer files are reported and skipped', () => {
  const { config, errors } = load({
    defaults: { a: 1 },
    layers: [resolve(here, 'fixtures/does-not-exist.json')],
    env: {},
    argv: [],
  });
  assert.equal(config.a, 1);
  assert.equal(errors.length, 1);
  assert.equal(errors[0].code, 'E_TYPE');
});

test('a malformed layer file does not stop later layers', () => {
  const { config, errors } = load({
    defaults: {},
    layers: [BASE, resolve(here, 'fixtures/malformed.json'), STAGING],
    env: {},
    argv: [],
  });
  assert.equal(config.db.pool.size, 16);
  assert.equal(errors.length, 1);
});

test('nested environment keys overlay a section without replacing it', () => {
  const config = envOverlay(
    { db: { host: 'h', pool: { size: 4 } } },
    { APP_DB__POOL__REGION: 'eu' },
    { prefix: 'APP_' },
  );
  assert.deepEqual(config, { db: { host: 'h', pool: { size: 4, region: 'eu' } } });
});

test('single underscores stay inside a key name', () => {
  const config = envOverlay({}, { APP_DB__POOL_MAX_SIZE: '9' }, { prefix: 'APP_' });
  assert.deepEqual(config, { db: { pool_max_size: '9' } });
});

test('deepMerge treats scalars and objects as replacements', () => {
  assert.deepEqual(deepMerge({ a: 1 }, { a: { nested: true } }), { a: { nested: true } });
  assert.deepEqual(deepMerge({ a: { nested: true } }, { a: 2 }), { a: 2 });
});

test('deepMerge does not keep references to the source subtrees', () => {
  const source = { a: { b: [1, 2] } };
  const merged = deepMerge({}, source);
  merged.a.b.push(3);
  assert.deepEqual(source.a.b, [1, 2]);
});

test('the document prototype is Object.prototype for ordinary input', () => {
  const { config } = load({ defaults: { a: { b: 1 } }, env: {}, argv: [] });
  assert.equal(Object.getPrototypeOf(config), Object.prototype);
  assert.equal(Object.getPrototypeOf(config.a), Object.prototype);
});
