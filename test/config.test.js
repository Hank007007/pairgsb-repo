import test from 'node:test';
import assert from 'node:assert/strict';

import { parseConfig, parseYaml } from '../src/config.js';

const SAMPLE = `
server:
  host: 127.0.0.1
  port: 8080
  request_timeout_ms: 15000
  headers_timeout_ms: 10000

jwt:
  secret: "dev-secret"
  issuer: auth.internal

introspection:
  endpoint: http://127.0.0.1:9100/introspect
  cache_ttl_ms: 30000
  timeout_ms: 2000

routes:
  - path: /api/public
    methods: [GET, POST]
    auth: public
    scopes: []
    upstream: http://127.0.0.1:9001
  - path: /api
    methods: [GET, POST, PUT, PATCH, DELETE]
    auth: jwt
    scopes: [docs:read]
    upstream: http://127.0.0.1:9000/
  - path: /events
    methods: ["*"]
    auth: public
    scopes: []
    upstream: http://127.0.0.1:9004
`;

test('yaml reader handles nested maps, sequences and flow sequences', () => {
  const doc = parseYaml(SAMPLE);
  assert.equal(doc.server.port, 8080);
  assert.equal(doc.jwt.secret, 'dev-secret');
  assert.deepEqual(doc.routes[0].methods, ['GET', 'POST']);
});

test('config exposes server, jwt and introspection settings', () => {
  const config = parseConfig(SAMPLE);
  assert.deepEqual(config.server, {
    host: '127.0.0.1',
    port: 8080,
    requestTimeoutMs: 15000,
    headersTimeoutMs: 10000,
  });
  assert.equal(config.jwt.secret, 'dev-secret');
  assert.equal(config.jwt.issuer, 'auth.internal');
  assert.equal(config.introspection.endpoint, 'http://127.0.0.1:9100/introspect');
  assert.equal(config.introspection.cacheTtlMs, 30000);
  assert.equal(config.introspection.timeoutMs, 2000);
});

test('routes keep their declared fields and are normalized for matching', () => {
  const config = parseConfig(SAMPLE);
  assert.equal(config.routes.length, 3);
  const [publicRoute, apiRoute, eventsRoute] = config.routes;
  assert.equal(publicRoute.path, '/api/public');
  assert.equal(publicRoute.auth, 'public');
  assert.deepEqual(publicRoute.scopes, []);
  assert.deepEqual(apiRoute.methods, ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']);
  assert.deepEqual(apiRoute.scopes, ['docs:read']);
  assert.equal(apiRoute.upstream, 'http://127.0.0.1:9000');
  assert.deepEqual(eventsRoute.methods, ['*']);
});

test('a route with an unknown auth mode is rejected', () => {
  assert.throws(
    () => parseConfig(SAMPLE.replace('auth: jwt', 'auth: mTLS')),
    /auth/,
  );
});
