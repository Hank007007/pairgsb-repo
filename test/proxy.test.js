'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const path = require('node:path');

const { createProxy } = require('../src/proxy');
const { loadVault } = require('../src/vault');
const {
  fixturePath,
  startHttpUpstream,
  startHttpsUpstream,
  startForwardProxy,
  makeTempDir,
  readAudit,
  requestThroughProxy,
  listen,
} = require('./helpers');

async function startProxy(options = {}) {
  const dir = makeTempDir();
  const proxy = createProxy({
    vault: loadVault(fixturePath('vault.json')),
    auditFile: path.join(dir, 'audit.jsonl'),
    caFile: fixturePath('ca.pem'),
    env: {},
    ...options,
  });
  const port = await listen(proxy);
  return { proxy, port, auditFile: path.join(dir, 'audit.jsonl') };
}

test('a request to a host with a rule arrives at the upstream with the credential injected', async (t) => {
  const upstream = await startHttpUpstream(() => ({ status: 200, body: 'hello' }));
  t.after(() => upstream.close());

  const { proxy, port } = await startProxy();
  t.after(() => proxy.close());

  const response = await requestThroughProxy(port, `http://localhost:${upstream.port}/v1/items`);
  assert.equal(response.status, 200);
  assert.equal(response.body, 'hello');

  assert.equal(upstream.seen.length, 1);
  const seen = upstream.seen[0];
  assert.equal(seen.method, 'GET');
  assert.equal(seen.url, '/v1/items');
  assert.equal(seen.headers.authorization, 'Bearer fixture-api-token-not-a-real-secret');
  assert.equal(seen.headers['x-egress-tenant'], 'example');
});

test('a caller supplied value cannot override the injected credential', async (t) => {
  const upstream = await startHttpUpstream(() => ({ status: 200, body: 'ok' }));
  t.after(() => upstream.close());

  const { proxy, port } = await startProxy();
  t.after(() => proxy.close());

  await requestThroughProxy(port, `http://localhost:${upstream.port}/`, {
    headers: { authorization: 'Bearer caller-supplied' },
  });

  assert.equal(upstream.seen[0].headers.authorization, 'Bearer fixture-api-token-not-a-real-secret');
});

test('a host without a rule is forwarded without any credential', async (t) => {
  const upstream = await startHttpUpstream(() => ({ status: 200, body: 'anon' }));
  t.after(() => upstream.close());

  const { proxy, port } = await startProxy();
  t.after(() => proxy.close());

  const response = await requestThroughProxy(port, `http://127.0.0.1:${upstream.port}/public`);
  assert.equal(response.status, 200);
  assert.equal(upstream.seen[0].headers.authorization, undefined);
  assert.equal(upstream.seen[0].headers['x-egress-tenant'], undefined);
});

test('request method and body are passed through unchanged', async (t) => {
  const upstream = await startHttpUpstream(() => ({ status: 201, body: 'created' }));
  t.after(() => upstream.close());

  const { proxy, port } = await startProxy();
  t.after(() => proxy.close());

  const response = await requestThroughProxy(port, `http://localhost:${upstream.port}/things`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name: 'widget' }),
  });

  assert.equal(response.status, 201);
  assert.equal(upstream.seen[0].method, 'POST');
  assert.equal(upstream.seen[0].body, JSON.stringify({ name: 'widget' }));
  assert.equal(upstream.seen[0].headers['content-type'], 'application/json');
});

test('an upstream 302 to another path on the same host is followed and keeps the credential', async (t) => {
  const upstream = await startHttpUpstream((record) => {
    if (record.url === '/start') {
      return { status: 302, headers: { location: '/finish' }, body: '' };
    }
    return { status: 200, body: 'finished' };
  });
  t.after(() => upstream.close());

  const { proxy, port } = await startProxy();
  t.after(() => proxy.close());

  const response = await requestThroughProxy(port, `http://localhost:${upstream.port}/start`);
  assert.equal(response.status, 200);
  assert.equal(response.body, 'finished');
  assert.equal(upstream.seen.length, 2);
  assert.equal(upstream.seen[1].url, '/finish');
  assert.equal(upstream.seen[1].headers.authorization, 'Bearer fixture-api-token-not-a-real-secret');
});

test('a redirect loop stops at the configured limit', async (t) => {
  const upstream = await startHttpUpstream(() => ({
    status: 302,
    headers: { location: '/again' },
    body: '',
  }));
  t.after(() => upstream.close());

  const { proxy, port } = await startProxy({ maxRedirects: 2 });
  t.after(() => proxy.close());

  const response = await requestThroughProxy(port, `http://localhost:${upstream.port}/loop`);
  assert.equal(response.status, 502);
  assert.equal(JSON.parse(response.body).error.code, 'ELOOP');
});

test('an https upstream is reached over a verified connection', async (t) => {
  const upstream = await startHttpsUpstream(() => ({ status: 200, body: 'secure' }));
  t.after(() => upstream.close());

  const previous = process.env.EGRESS_TLS_INSECURE;
  delete process.env.EGRESS_TLS_INSECURE;
  t.after(() => {
    if (previous !== undefined) process.env.EGRESS_TLS_INSECURE = previous;
  });

  const { proxy, port } = await startProxy();
  t.after(() => proxy.close());

  const response = await requestThroughProxy(port, `https://localhost:${upstream.port}/secure`);
  assert.equal(response.status, 200);
  assert.equal(response.body, 'secure');
  assert.equal(upstream.seen[0].headers.authorization, 'Bearer fixture-api-token-not-a-real-secret');
});

test('the audit file records host, path, status and key id for each hop', async (t) => {
  const upstream = await startHttpUpstream(() => ({ status: 204, body: '' }));
  t.after(() => upstream.close());

  const { proxy, port, auditFile } = await startProxy();
  t.after(() => proxy.close());

  await requestThroughProxy(port, `http://localhost:${upstream.port}/audit/me?token=abc`);
  await new Promise((resolve) => setTimeout(resolve, 20));

  const records = readAudit(auditFile);
  assert.equal(records.length, 1);
  const record = records[0];
  assert.equal(record.host, 'localhost');
  assert.equal(record.path, '/audit/me?token=…');
  assert.equal(record.status, 204);
  assert.equal(record.keyId, 'svc-example-api');
  assert.equal(typeof record.durationMs, 'number');
  assert.equal(typeof record.ts, 'string');
});

test('a configured upstream proxy receives the request in absolute form', async (t) => {
  const origin = await startHttpUpstream(() => ({ status: 200, body: 'origin' }));
  const forward = await startForwardProxy();
  t.after(() => origin.close());
  t.after(() => forward.close());

  const { proxy, port } = await startProxy({ env: { HTTP_PROXY: forward.url } });
  t.after(() => proxy.close());

  const response = await requestThroughProxy(port, `http://localhost:${origin.port}/proxied/path`);
  assert.equal(response.status, 200);
  assert.equal(response.body, 'origin');

  assert.equal(forward.seen.length, 1);
  assert.equal(forward.seen[0].url, `http://localhost:${origin.port}/proxied/path`);
  assert.equal(forward.seen[0].headers.authorization, 'Bearer fixture-api-token-not-a-real-secret');
  assert.equal(origin.seen.length, 1);
  assert.equal(origin.seen[0].url, '/proxied/path');
});

test('a scheme-specific proxy variable wins over the catch-all one', async (t) => {
  const origin = await startHttpUpstream(() => ({ status: 200, body: 'origin' }));
  const forward = await startForwardProxy();
  const catchAll = await startForwardProxy();
  t.after(() => origin.close());
  t.after(() => forward.close());
  t.after(() => catchAll.close());

  const { proxy, port } = await startProxy({
    env: { HTTP_PROXY: forward.url, ALL_PROXY: catchAll.url },
  });
  t.after(() => proxy.close());

  await requestThroughProxy(port, `http://localhost:${origin.port}/either`);
  assert.equal(forward.seen.length, 1);
  assert.equal(catchAll.seen.length, 0);
});

test('an unreachable upstream proxy is reported as an upstream error', async (t) => {
  const { proxy, port } = await startProxy({ env: { HTTP_PROXY: 'http://127.0.0.1:1' } });
  t.after(() => proxy.close());

  const response = await requestThroughProxy(port, 'http://localhost:1/nowhere');
  assert.equal(response.status, 502);
  assert.equal(JSON.parse(response.body).error.code, 'EUPSTREAM');
});

test('an unsupported request target is refused', async (t) => {
  const { proxy, port } = await startProxy();
  t.after(() => proxy.close());

  const response = await new Promise((resolve, reject) => {
    const req = http.request(
      { host: '127.0.0.1', port, method: 'GET', path: 'ftp://example.com/file' },
      (res) => {
        const chunks = [];
        res.on('data', (c) => chunks.push(c));
        res.on('end', () => resolve({ status: res.statusCode, body: Buffer.concat(chunks).toString('utf8') }));
      }
    );
    req.on('error', reject);
    req.end();
  });

  assert.equal(response.status, 502);
  assert.equal(JSON.parse(response.body).error.code, 'EUNSUPPORTEDSCHEME');
});
