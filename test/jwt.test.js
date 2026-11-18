import test from 'node:test';
import assert from 'node:assert/strict';
import { generateKeyPair, SignJWT } from 'jose';
import { verifyToken, TokenError, loadKeySet } from '../src/jwt.js';
import { loadTestKeySet, signToken } from '../test-support/helpers.js';

const policy = {
  issuer: 'https://auth.internal.hopguard',
  audience: 'hopguard-edge',
  algorithms: ['RS256'],
  clockToleranceSeconds: 30,
};

test('accepts a well-formed token and exposes the subject and roles', async () => {
  const keys = await loadTestKeySet();
  const token = await signToken({ sub: 'user-42', roles: ['operator', 'auditor'] });

  const identity = await verifyToken(token, { keys, token: policy });

  assert.equal(identity.sub, 'user-42');
  assert.deepEqual(identity.roles, ['operator', 'auditor']);
  assert.deepEqual(identity.payload.scope, ['orders:read']);
});

test('rejects a token signed by a key outside the key set', async () => {
  const keys = await loadTestKeySet();
  const { privateKey } = await generateKeyPair('RS256', { extractable: true });
  const now = Math.floor(Date.now() / 1000);
  const token = await new SignJWT({ sub: 'user-42', exp: now + 600 })
    .setProtectedHeader({ alg: 'RS256', kid: 'local-test-1' })
    .sign(privateKey);

  await assert.rejects(
    verifyToken(token, { keys, token: policy }),
    (error) => error instanceof TokenError && error.reason === 'signature',
  );
});

test('rejects a value that is not a compact JWT', async () => {
  const keys = await loadTestKeySet();

  await assert.rejects(
    verifyToken('not-a-token', { keys, token: policy }),
    (error) => error instanceof TokenError && error.reason === 'malformed',
  );
});

test('rejects a token that does not identify a subject', async () => {
  const keys = await loadKeySet(new URL('../keys/jwks.json', import.meta.url));
  const token = await signToken({ sub: undefined });

  await assert.rejects(
    verifyToken(token, { keys, token: policy }),
    (error) => error instanceof TokenError && error.reason === 'claims',
  );
});
