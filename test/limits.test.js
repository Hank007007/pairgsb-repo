'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { isRegexSafe } = require('../src/policy/limits');

test('ordinary patterns are considered safe', () => {
  assert.deepEqual(isRegexSafe('^[a-z0-9._-]+$'), { safe: true, reason: null });
  assert.deepEqual(isRegexSafe('\\d{3}-\\d{4}'), { safe: true, reason: null });
});

test('over-long patterns are reported as unsafe', () => {
  const { safe, reason } = isRegexSafe('a'.repeat(600));
  assert.equal(safe, false);
  assert.match(reason, /length/);
});

test('lookaround is reported as unsafe', () => {
  const { safe } = isRegexSafe('foo(?=bar)');
  assert.equal(safe, false);
});