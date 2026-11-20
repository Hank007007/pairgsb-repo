import test from 'node:test';
import assert from 'node:assert/strict';

import { buildHeaders, upstreamTarget } from '../src/proxy.js';

test('buildHeaders adds the gateway identity headers', () => {
  const headers = buildHeaders(
    { 'content-type': 'application/json' },
    { user: 'alice', scopes: 'docs:read admin:write', tokenId: 'tok-1' },
  );
  assert.equal(headers['x-auth-user'], 'alice');
  assert.equal(headers['x-auth-scopes'], 'docs:read admin:write');
  assert.equal(headers['x-auth-token-id'], 'tok-1');
  assert.equal(headers['content-type'], 'application/json');
});

test('buildHeaders omits the token id when there is none', () => {
  const headers = buildHeaders({}, { user: 'alice', scopes: '', tokenId: null });
  assert.equal(headers['x-auth-user'], 'alice');
  assert.equal(headers['x-auth-scopes'], '');
  assert.equal(headers['x-auth-token-id'], undefined);
});

test('buildHeaders carries extra gateway headers', () => {
  const headers = buildHeaders({}, null, { 'x-request-id': 'req-1' });
  assert.equal(headers['x-request-id'], 'req-1');
});

test('upstreamTarget joins the upstream base with the request target', () => {
  assert.equal(upstreamTarget('http://127.0.0.1:9000', '/api/docs?page=2').href, 'http://127.0.0.1:9000/api/docs?page=2');
  assert.equal(upstreamTarget('http://127.0.0.1:9000/base', '/api/docs').href, 'http://127.0.0.1:9000/api/docs');
});
