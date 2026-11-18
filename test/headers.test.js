import { test } from 'node:test';
import assert from 'node:assert/strict';
import { forwardHeaders } from '../src/proxy.js';

test('keeps ordinary request headers', () => {
  const headers = forwardHeaders(
    { accept: 'application/json', 'x-request-id': 'abc', 'content-type': 'application/json' },
    null,
  );

  assert.equal(headers.accept, 'application/json');
  assert.equal(headers['x-request-id'], 'abc');
  assert.equal(headers['content-type'], 'application/json');
});

test('writes the authenticated identity onto the upstream request', () => {
  const headers = forwardHeaders(
    { accept: 'text/plain' },
    { sub: 'user-7', roles: ['operator', 'auditor'] },
  );

  assert.equal(headers['x-auth-sub'], 'user-7');
  assert.equal(headers['x-auth-roles'], 'operator,auditor');
});

test('does not forward hop-by-hop headers', () => {
  const headers = forwardHeaders(
    { connection: 'keep-alive', te: 'trailers', 'x-request-id': 'abc' },
    null,
  );

  assert.equal(headers.connection, undefined);
  assert.equal(headers.te, undefined);
  assert.equal(headers['x-request-id'], 'abc');
});
