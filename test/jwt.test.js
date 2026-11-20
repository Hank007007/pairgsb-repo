import test from 'node:test';
import assert from 'node:assert/strict';

import { bearerToken, signToken, verifyToken } from '../src/jwt.js';

const SECRET = 'test-secret';
const NOW = 1_700_000_000_000;
const nowSeconds = Math.floor(NOW / 1000);

test('signToken produces a token verifyToken accepts', () => {
  const token = signToken({ sub: 'alice', iss: 'auth.internal', exp: nowSeconds + 60 }, SECRET);
  const result = verifyToken(token, { secret: SECRET, issuer: 'auth.internal', now: NOW });
  assert.equal(result.ok, true);
  assert.equal(result.claims.sub, 'alice');
});

test('verifyToken rejects a missing token', () => {
  assert.deepEqual(verifyToken('', { secret: SECRET }), { ok: false, reason: 'missing-token' });
  assert.deepEqual(verifyToken(null, { secret: SECRET }), { ok: false, reason: 'missing-token' });
});

test('verifyToken rejects a token signed with another secret', () => {
  const token = signToken({ sub: 'alice', exp: nowSeconds + 60 }, 'other-secret');
  const result = verifyToken(token, { secret: SECRET, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'bad-signature');
});

test('verifyToken rejects an expired token', () => {
  const token = signToken({ sub: 'alice', exp: nowSeconds - 1 }, SECRET);
  const result = verifyToken(token, { secret: SECRET, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'expired');
});

test('verifyToken rejects a token from another issuer', () => {
  const token = signToken({ sub: 'alice', iss: 'elsewhere', exp: nowSeconds + 60 }, SECRET);
  const result = verifyToken(token, { secret: SECRET, issuer: 'auth.internal', now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'bad-issuer');
});

test('bearerToken reads a bearer credential', () => {
  assert.equal(bearerToken('Bearer abc.def.ghi'), 'abc.def.ghi');
  assert.equal(bearerToken('bearer abc'), 'abc');
  assert.equal(bearerToken('Basic abc'), null);
  assert.equal(bearerToken(undefined), null);
});
