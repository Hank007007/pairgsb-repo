'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { seal, open, errors } = require('../src');

const KEY = 'roundtrip-test-key';

const SESSION = {
  sub: 'user-1234',
  roles: ['reader', 'writer'],
  profile: { displayName: 'Ada Lovelace', locale: 'en-GB' },
  counters: { logins: 7, version: 3 },
  flags: { beta: true, suspended: false },
  note: null,
};

test('a sealed session opens back to an equal object', () => {
  const token = seal(SESSION, { key: KEY });

  assert.equal(typeof token, 'string');
  assert.equal(token.split('.').length, 2);

  const restored = open(token, { key: KEY });
  assert.deepEqual(restored, SESSION);
  assert.notEqual(restored, SESSION);
});

test('an envelope with an altered signature is rejected', () => {
  const token = seal(SESSION, { key: KEY });
  const [payload, mac] = token.split('.');
  const flipped = (mac[0] === 'A' ? 'B' : 'A') + mac.slice(1);

  assert.throws(
    () => open(`${payload}.${flipped}`, { key: KEY }),
    (error) => error instanceof errors.EnvelopeSignatureError
  );
});
