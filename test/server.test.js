import test from 'node:test';
import assert from 'node:assert/strict';

import { parseConfig } from '../src/config.js';
import { createServer } from '../src/server.js';
import { signToken } from '../src/jwt.js';
import { recordingHandler, request, startServer } from './helpers.js';

const SECRET = 'integration-secret';
const NOW = 1_700_000_000_000;
const nowSeconds = Math.floor(NOW / 1000);

/**
 * Start one recording upstream per route, a stub authorization service and the
 * gateway in front of them. Returns handles the tests can assert against.
 */
async function harness(t) {
  const publicUpstream = recordingHandler();
  const apiUpstream = recordingHandler();
  const adminUpstream = recordingHandler();
  const reportsUpstream = recordingHandler();
  const eventsUpstream = recordingHandler();
  const healthUpstream = recordingHandler();

  const publicServer = await startServer(publicUpstream);
  const apiServer = await startServer(apiUpstream);
  const adminServer = await startServer(adminUpstream);
  const reportsServer = await startServer(reportsUpstream);
  const eventsServer = await startServer(eventsUpstream);
  const healthServer = await startServer(healthUpstream);

  const introspectionCalls = [];
  const authServer = await startServer((req, res, body) => {
    const token = new URLSearchParams(body.toString('utf8')).get('token');
    introspectionCalls.push(token);
    const known = {
      'admin-token': { active: true, sub: 'dana', scope: 'admin:write', token_id: 'tok-admin' },
      'reports-token': { active: true, sub: 'erin', scope: 'reports:read', token_id: 'tok-reports' },
    }[token] ?? { active: false };
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(known));
  });

  t.after(async () => {
    await Promise.all([
      publicServer.close(),
      apiServer.close(),
      adminServer.close(),
      reportsServer.close(),
      eventsServer.close(),
      healthServer.close(),
      authServer.close(),
    ]);
  });

  const config = parseConfig(`
server:
  host: 127.0.0.1
  port: 0
  request_timeout_ms: 5000
  headers_timeout_ms: 5000

jwt:
  secret: ${SECRET}
  issuer: auth.internal

introspection:
  endpoint: ${authServer.url}/introspect
  cache_ttl_ms: 30000
  timeout_ms: 1000

routes:
  - path: /api/public
    methods: [GET, POST]
    auth: public
    scopes: []
    upstream: ${publicServer.url}
  - path: /api
    methods: [GET, POST, PUT, PATCH, DELETE]
    auth: jwt
    scopes: [docs:read]
    upstream: ${apiServer.url}
  - path: /api/admin
    methods: [GET, POST, DELETE]
    auth: introspect
    scopes: [admin:write]
    upstream: ${adminServer.url}
  - path: /reports
    methods: [GET]
    auth: introspect
    scopes: [reports:read]
    upstream: ${reportsServer.url}
  - path: /events
    methods: ["*"]
    auth: public
    scopes: []
    upstream: ${eventsServer.url}
  - path: /health
    methods: [GET]
    auth: public
    scopes: []
    upstream: ${healthServer.url}
`);

  const server = createServer(config, { now: () => NOW });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise((resolve) => server.close(resolve)));
  const base = `http://127.0.0.1:${server.address().port}`;

  const jwt = (claims) => signToken(claims, SECRET);

  return {
    base,
    jwt,
    upstreams: { publicUpstream, apiUpstream, adminUpstream, reportsUpstream, eventsUpstream, healthUpstream },
    introspectionCalls,
  };
}

test('an anonymous request reaches a public route', async (t) => {
  const h = await harness(t);
  const res = await request(`${h.base}/api/public/status`);
  assert.equal(res.status, 200);
  assert.equal(h.upstreams.publicUpstream.seen.length, 1);
});

test('a jwt route accepts a token carrying the required scope', async (t) => {
  const h = await harness(t);
  const token = h.jwt({ sub: 'alice', iss: 'auth.internal', exp: nowSeconds + 60, scope: 'docs:read docs:write' });
  const res = await request(`${h.base}/api/docs`, { headers: { authorization: `Bearer ${token}` } });
  assert.equal(res.status, 200);
  assert.equal(h.upstreams.apiUpstream.seen.length, 1);
});

test('a jwt route rejects a request without a token', async (t) => {
  const h = await harness(t);
  const res = await request(`${h.base}/api/docs`);
  assert.equal(res.status, 401);
  assert.equal(h.upstreams.apiUpstream.seen.length, 0);
});

test('a jwt route rejects an expired token', async (t) => {
  const h = await harness(t);
  const token = h.jwt({ sub: 'alice', iss: 'auth.internal', exp: nowSeconds - 5, scope: 'docs:read' });
  const res = await request(`${h.base}/api/docs`, { headers: { authorization: `Bearer ${token}` } });
  assert.equal(res.status, 401);
});

test('a jwt route rejects a caller whose token lacks the required scope', async (t) => {
  const h = await harness(t);
  const token = h.jwt({ sub: 'alice', iss: 'auth.internal', exp: nowSeconds + 60, scope: 'reports:read' });
  const res = await request(`${h.base}/api/docs`, { headers: { authorization: `Bearer ${token}` } });
  assert.equal(res.status, 403);
  assert.equal(h.upstreams.apiUpstream.seen.length, 0);
});

test('an introspect route accepts a token the authorization service reports active', async (t) => {
  const h = await harness(t);
  const res = await request(`${h.base}/reports/quarterly`, { headers: { authorization: 'Bearer reports-token' } });
  assert.equal(res.status, 200);
  assert.equal(h.upstreams.reportsUpstream.seen.length, 1);
  assert.deepEqual(h.introspectionCalls, ['reports-token']);
});

test('an introspect route rejects a token the authorization service reports inactive', async (t) => {
  const h = await harness(t);
  const res = await request(`${h.base}/reports/quarterly`, { headers: { authorization: 'Bearer revoked-token' } });
  assert.equal(res.status, 401);
  assert.equal(h.upstreams.reportsUpstream.seen.length, 0);
});

test('the gateway forwards its own identity headers to the upstream', async (t) => {
  const h = await harness(t);
  const token = h.jwt({ sub: 'alice', iss: 'auth.internal', exp: nowSeconds + 60, scope: 'docs:read' });
  await request(`${h.base}/api/docs`, { headers: { authorization: `Bearer ${token}` } });

  const seen = h.upstreams.apiUpstream.seen[0].headers;
  assert.equal(seen['x-auth-user'], 'alice');
  assert.equal(seen['x-auth-scopes'], 'docs:read');
});

test('an unknown path is not forwarded', async (t) => {
  const h = await harness(t);
  const res = await request(`${h.base}/nowhere`);
  assert.equal(res.status, 404);
});

test('a method the route does not list is rejected', async (t) => {
  const h = await harness(t);
  const token = h.jwt({ sub: 'alice', iss: 'auth.internal', exp: nowSeconds + 60, scope: 'docs:read' });
  const res = await request(`${h.base}/api/docs`, {
    method: 'TRACE',
    headers: { authorization: `Bearer ${token}` },
  });
  assert.equal(res.status, 405);
});

test('a health check needs no credential', async (t) => {
  const h = await harness(t);
  const res = await request(`${h.base}/health`);
  assert.equal(res.status, 200);
  assert.equal(h.upstreams.healthUpstream.seen.length, 1);
});
