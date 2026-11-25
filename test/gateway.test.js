import test from 'node:test';
import assert from 'node:assert/strict';

import { SchemaRegistry } from '../lib/schema/registry.js';
import { resolveType } from '../lib/schema/resolve.js';
import { signRequest, verifyRequest } from '../lib/gateway/verify.js';
import { handleRequest } from '../lib/gateway/server.js';
import { loadMessage, loadSnapshot } from '../tools/fixtures.js';

const SECRET = 'test-secret-please-rotate';

function context() {
  const registry = new SchemaRegistry(loadSnapshot('telemetry.v1.json'));
  return {
    registry,
    secret: SECRET,
    resolve(type, version) {
      return resolveType(registry, type, version);
    },
  };
}

test('verifyRequest accepts a request signed with the fixed key', () => {
  const ctx = context();
  const body = loadMessage('telemetry-basic.bin');
  const signature = signRequest(SECRET, body);

  const { message, version } = verifyRequest({ body, signature, type: 'Telemetry' }, ctx);

  assert.equal(version, 1);
  assert.equal(message.device_id, 'edge-1');
  assert.equal(message.sequence, 42);
});

test('verifyRequest rejects a request whose signature belongs to another key', () => {
  const ctx = context();
  const body = loadMessage('telemetry-basic.bin');
  const signature = signRequest('some-other-key', body);

  assert.throws(
    () => verifyRequest({ body, signature, type: 'Telemetry' }, ctx),
    (err) => err.code === 'signature_invalid',
  );
});

test('handleRequest dispatches a valid request to the router', () => {
  const ctx = context();
  const body = loadMessage('telemetry-basic.bin');
  const signature = signRequest(SECRET, body);
  const seen = [];

  const response = handleRequest(
    { body, signature, type: 'Telemetry', headers: { 'x-trace-id': 'abc' } },
    {
      ...ctx,
      router(message, info) {
        seen.push({ message, info });
        return 'accepted';
      },
    },
  );

  assert.equal(response.status, 200);
  assert.equal(response.body.version, 1);
  assert.equal(response.body.message.device_id, 'edge-1');
  assert.equal(response.body.routed, 'accepted');
  assert.equal(seen.length, 1);
  assert.equal(seen[0].info.headers['x-trace-id'], 'abc');
});
