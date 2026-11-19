import test from 'node:test';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

import { load, applyPatch, deepMerge } from '../src/index.js';

const here = dirname(fileURLToPath(import.meta.url));
const BASE = resolve(here, 'fixtures/base.json');
const STAGING = resolve(here, 'fixtures/staging.json');

const DEFAULTS = {
  service: { name: 'orders', port: 8080 },
  db: { host: 'db.internal', pool: { size: 4, timeoutMs: 1000 } },
  features: ['beta'],
  logging: { level: 'info' },
};

test('defaults are returned when no other layer is supplied', () => {
  const { config, errors } = load({ defaults: DEFAULTS, env: {}, argv: [] });
  assert.deepEqual(errors, []);
  assert.deepEqual(config, DEFAULTS);
  assert.notEqual(config, DEFAULTS);
});

test('base.json overrides defaults key by key', () => {
  const { config, errors } = load({
    defaults: DEFAULTS,
    layers: [BASE],
    env: {},
    argv: [],
  });
  assert.deepEqual(errors, []);
  assert.equal(config.db.host, 'db.internal');
  assert.equal(config.service.port, 8080);
  assert.deepEqual(config.features, ['beta']);
});

test('overlays win over base.json', () => {
  const { config, errors } = load({
    defaults: DEFAULTS,
    layers: [BASE, STAGING],
    env: {},
    argv: [],
  });
  assert.deepEqual(errors, []);
  assert.equal(config.db.pool.size, 16);
  assert.equal(config.db.pool.timeoutMs, 1000);
  assert.equal(config.logging.level, 'trace');
});

test('arrays are replaced, not concatenated, across layers', () => {
  const { config } = load({
    defaults: { features: ['beta', 'alpha'] },
    layers: [STAGING],
    env: {},
    argv: [],
  });
  assert.deepEqual(config.features, ['beta', 'audit']);
});

test('environment variables override layers and create nested keys', () => {
  const { config, errors } = load({
    defaults: DEFAULTS,
    layers: [BASE, STAGING],
    env: { APP_DB__POOL__REGION: 'eu-west-1' },
    prefix: 'APP_',
    argv: [],
  });
  assert.deepEqual(errors, []);
  assert.equal(config.db.pool.region, 'eu-west-1');
  assert.equal(config.db.pool.size, 16);
});

test('environment variables without the prefix are ignored', () => {
  const { config } = load({
    defaults: DEFAULTS,
    env: { OTHER_DB__HOST: 'ignored.internal' },
    prefix: 'APP_',
    argv: [],
  });
  assert.equal(config.db.host, 'db.internal');
});

test('--set entries win over environment variables', () => {
  const { config, errors } = load({
    defaults: DEFAULTS,
    layers: [BASE],
    env: { APP_LOGGING__LEVEL: 'warn' },
    prefix: 'APP_',
    argv: ['--set', 'logging.level=error', '--set', 'service.port=9090'],
  });
  assert.deepEqual(errors, []);
  assert.equal(config.logging.level, 'error');
  assert.equal(config.service.port, 9090);
});

test('a later --set entry for the same path wins', () => {
  const { config } = load({
    defaults: { mode: 'a' },
    env: {},
    argv: ['--set', 'mode=b', '--set', 'mode=c'],
  });
  assert.equal(config.mode, 'c');
});

test('load does not mutate its inputs', () => {
  const defaults = structuredClone(DEFAULTS);
  const { config } = load({ defaults, layers: [BASE, STAGING], env: {}, argv: [] });
  assert.deepEqual(defaults, DEFAULTS);
  config.db.pool.size = 999;
  assert.equal(defaults.db.pool.size, 4);
});

test('deepMerge merges nested plain objects and replaces arrays', () => {
  const target = deepMerge({ a: { b: 1 }, list: [1, 2] }, { a: { c: 2 }, list: [3] });
  assert.deepEqual(target, { a: { b: 1, c: 2 }, list: [3] });
});

test('applyPatch returns a new document and leaves the input untouched', () => {
  const source = { db: { pool: { size: 4 } }, names: ['a'] };
  const { document, errors } = applyPatch(source, [
    { op: 'replace', path: '/db/pool/size', value: 8 },
    { op: 'add', path: '/names/-', value: 'b' },
  ]);
  assert.deepEqual(errors, []);
  assert.equal(document.db.pool.size, 8);
  assert.deepEqual(document.names, ['a', 'b']);
  assert.equal(source.db.pool.size, 4);
  assert.deepEqual(source.names, ['a']);
});

test('applyPatch supports remove, copy, move and test', () => {
  const { document, errors } = applyPatch(
    { a: 1, b: { c: 2 }, list: ['x', 'y'] },
    [
      { op: 'test', path: '/a', value: 1 },
      { op: 'copy', from: '/b', path: '/copy' },
      { op: 'move', from: '/a', path: '/moved' },
      { op: 'remove', path: '/list/0' },
    ],
  );
  assert.deepEqual(errors, []);
  assert.deepEqual(document, { b: { c: 2 }, copy: { c: 2 }, moved: 1, list: ['y'] });
});

test('applyPatch reports an unsupported operation without throwing', () => {
  const { errors } = applyPatch({ a: 1 }, [{ op: 'merge', path: '/a', value: 2 }]);
  assert.equal(errors.length, 1);
  assert.equal(errors[0].code, 'E_PATCH_OP');
});
