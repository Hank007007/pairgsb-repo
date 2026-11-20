import test from 'node:test';
import assert from 'node:assert/strict';

import { createIntrospector, introspectRemote } from '../src/introspect.js';
import { startServer } from './helpers.js';

function introspectionStub(responder) {
  return startServer((req, res, body) => {
    const token = new URLSearchParams(body.toString('utf8')).get('token');
    const payload = responder(token);
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(payload));
  });
}

test('introspectRemote sends the token and parses an active response', async (t) => {
  let received = null;
  const stub = await introspectionStub((token) => {
    received = token;
    return { active: true, sub: 'alice', scope: 'docs:read', exp: 2_000_000_000 };
  });
  t.after(() => stub.close());

  const result = await introspectRemote('opaque-token', { endpoint: `${stub.url}/introspect`, timeoutMs: 1000 });
  assert.equal(result.ok, true);
  assert.equal(received, 'opaque-token');
  assert.equal(result.payload.sub, 'alice');
});

test('introspectRemote reports a non-2xx response as a failure', async (t) => {
  const stub = await startServer((req, res) => {
    res.writeHead(503);
    res.end('unavailable');
  });
  t.after(() => stub.close());

  const result = await introspectRemote('opaque-token', { endpoint: `${stub.url}/introspect`, timeoutMs: 1000 });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'status-503');
});

test('createIntrospector answers a repeated lookup without a second round trip', async (t) => {
  let calls = 0;
  const stub = await introspectionStub(() => {
    calls += 1;
    return { active: true, sub: 'alice', scope: 'docs:read', exp: 2_000_000_000 };
  });
  t.after(() => stub.close());

  const introspector = createIntrospector({ endpoint: `${stub.url}/introspect`, timeoutMs: 1000, cacheTtlMs: 30000 });
  const first = await introspector.lookup('token-a');
  const second = await introspector.lookup('token-a');

  assert.equal(first.ok, true);
  assert.equal(second.ok, true);
  assert.equal(second.payload.sub, 'alice');
  assert.equal(calls, 1);
});

test('createIntrospector keeps distinct tokens apart', async (t) => {
  const stub = await introspectionStub((token) => ({
    active: true,
    sub: token === 'token-a' ? 'alice' : 'bob',
    scope: 'docs:read',
  }));
  t.after(() => stub.close());

  const introspector = createIntrospector({ endpoint: `${stub.url}/introspect`, timeoutMs: 1000, cacheTtlMs: 30000 });
  const a = await introspector.lookup('token-a');
  const b = await introspector.lookup('token-b');

  assert.equal(a.payload.sub, 'alice');
  assert.equal(b.payload.sub, 'bob');
  assert.equal(introspector.size(), 2);
});
