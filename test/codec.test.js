'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { decodeGrant, encodeGrant, PREFIX } = require('../src/codec');

test('codec: encode and decode round-trip a grant payload', () => {
  const grant = {
    id: 'abc12345',
    tenantId: 'tenant-a',
    path: '/tenant-a/docs',
    actions: ['read'],
    ttlMs: 60_000,
    issuedAtMs: 1_700_000_000_000,
    expiresAtMs: 1_700_000_060_000,
  };

  const token = encodeGrant(grant);
  assert.equal(typeof token, 'string');
  assert.ok(token.startsWith(`${PREFIX}.`));

  assert.deepEqual(decodeGrant(token), grant);
});

test('codec: unicode payloads survive a round-trip', () => {
  const grant = { id: 'x', path: '/tenant-a/文档/报告.txt' };
  assert.deepEqual(decodeGrant(encodeGrant(grant)), grant);
});

test('codec: decode rejects a token without a prefix separator', () => {
  assert.throws(() => decodeGrant('not-a-token'), /separator/);
});

test('codec: decode rejects an unknown prefix', () => {
  assert.throws(() => decodeGrant(`other.${Buffer.from('{}').toString('base64url')}`), /prefix/);
});

test('codec: decode rejects a payload that is not an object', () => {
  const encoded = Buffer.from('[1,2]', 'utf8').toString('base64url');
  assert.throws(() => decodeGrant(`${PREFIX}.${encoded}`), /object/);
});
