import test from 'node:test';
import assert from 'node:assert/strict';

import { hmac, resolveKey, signaturesMatch } from '../src/sign.js';

test('hmac produces lowercase hex sha256', () => {
  const sig = hmac('demo-secret', 'hello');
  assert.match(sig, /^[0-9a-f]{64}$/);
  assert.equal(sig, hmac('demo-secret', 'hello'));
});

test('hmac depends on the secret and the message', () => {
  assert.notEqual(hmac('demo-secret', 'hello'), hmac('other-secret', 'hello'));
  assert.notEqual(hmac('demo-secret', 'hello'), hmac('demo-secret', 'hello '));
});

test('hmac rejects an empty secret', () => {
  assert.throws(() => hmac('', 'hello'), /secret/);
});

test('signaturesMatch accepts equal strings and rejects different ones', () => {
  assert.equal(signaturesMatch('abc', 'abc'), true);
  assert.equal(signaturesMatch('abc', 'abd'), false);
});

test('resolveKey returns the tenant entry when present', () => {
  const keyring = {
    demo: [{ version: 1, secret: 'demo-secret' }],
    global: [{ version: 1, secret: 'global-secret' }],
  };
  assert.equal(resolveKey(keyring, 'demo')[0].secret, 'demo-secret');
});
