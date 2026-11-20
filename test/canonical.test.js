import test from 'node:test';
import assert from 'node:assert/strict';

import {
  canonicalKey,
  canonicalQuery,
  encodePath,
  parseQuery,
  stripProtocolParams,
} from '../src/canonical.js';

test('parses a query string into decoded pairs', () => {
  assert.deepEqual(parseQuery('a=1&b=hello%20world'), { a: '1', b: 'hello world' });
  assert.deepEqual(parseQuery(''), {});
  assert.deepEqual(parseQuery(undefined), {});
});

test('strips protocol parameters and sig from a query object', () => {
  const raw = parseQuery('a=1&X-Seal-Method=GET&X-Seal-Expires=10&sig=abc');
  assert.deepEqual(stripProtocolParams(raw), { a: '1' });
});

test('canonical query orders pairs by name', () => {
  assert.equal(canonicalQuery({ b: '2', a: '1' }), 'a=1&b=2');
});

test('canonical query is empty for an empty query', () => {
  assert.equal(canonicalQuery({}), '');
  assert.equal(canonicalQuery(undefined), '');
  assert.equal(canonicalQuery(''), '');
});

test('canonical query accepts a raw query string', () => {
  assert.equal(canonicalQuery('b=2&a=1'), 'a=1&b=2');
});

test('canonical key leaves simple keys untouched', () => {
  assert.equal(canonicalKey('reports/2024/q1.pdf'), 'reports/2024/q1.pdf');
  assert.equal(canonicalKey('images/logo.png'), 'images/logo.png');
});

test('encodePath percent-encodes each segment and keeps slashes', () => {
  assert.equal(encodePath('assets', 'reports/2024 q1.pdf'), '/assets/reports/2024%20q1.pdf');
  assert.equal(encodePath('media', 'images/logo.png'), '/media/images/logo.png');
});
