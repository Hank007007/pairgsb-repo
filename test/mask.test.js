'use strict';

const { buildMatcher, mask } = require('../lib/mask');

const test = require('node:test');
const assert = require('node:assert/strict');

test('buildMatcher orders secrets longest first', () => {
  const matcher = buildMatcher(['abc', 'abcdef']);

  assert.deepEqual(matcher.secrets, ['abcdef', 'abc']);

  const hit = matcher.matchAt(Buffer.from('abcdef-end'), 0);
  assert.equal(hit.secret, 'abcdef');
  assert.equal(hit.length, 6);
});

test('mask replaces a secret contained in a single chunk', () => {
  const secrets = ['s3cr3t'];
  const output = mask(secrets, 'connecting with s3cr3t now');

  assert.equal(output, 'connecting with [REDACTED] now');
});
