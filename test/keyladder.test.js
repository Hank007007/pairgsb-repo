import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createService, createFixedClock, parseKey } from '../src/index.js';

function temporaryDataFile(t) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'keyladder-test-'));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  return path.join(directory, 'keys.json');
}

function quietService(overrides = {}) {
  return createService({
    logger: { debug() {}, info() {}, warn() {}, error() {} },
    ...overrides,
  });
}

test('issuing a key returns a well-formed key and stores the record', (t) => {
  const dataFile = temporaryDataFile(t);
  const service = quietService({
    dataFile,
    clock: createFixedClock('2026-11-18T00:00:00.000Z'),
  });

  const issued = service.issue({
    ownerId: 'checkout-team',
    scopes: ['orders:read'],
    env: 'prod',
  });

  const parsed = parseKey(issued.key);
  assert.ok(parsed, 'key matches kl_<env>_<id8>_<secret>');
  assert.equal(parsed.env, 'prod');
  assert.equal(parsed.id8, issued.id8);

  const record = service.store.getById(issued.id8);
  assert.equal(record.ownerId, 'checkout-team');
  assert.deepEqual(record.scopes, ['orders:read']);
  assert.equal(record.env, 'prod');
  assert.equal(record.revoked, false);

  assert.ok(fs.existsSync(dataFile));
});

test('a freshly issued key verifies for the matching environment', (t) => {
  const service = quietService({
    dataFile: temporaryDataFile(t),
    clock: createFixedClock('2026-11-18T00:00:00.000Z'),
  });

  const issued = service.issue({
    ownerId: 'search-team',
    scopes: ['search:read', 'search:write'],
    env: 'staging',
  });

  const result = service.verify(issued.key, { env: 'staging' });

  assert.equal(result.ok, true);
  assert.equal(result.ownerId, 'search-team');
  assert.equal(result.env, 'staging');
  assert.deepEqual(result.scopes, ['search:read', 'search:write']);
});

test('a revoked key is rejected', (t) => {
  const clock = createFixedClock('2026-11-18T00:00:00.000Z');
  const service = quietService({ dataFile: temporaryDataFile(t), clock });

  const issued = service.issue({ ownerId: 'billing-team', scopes: ['invoices:read'] });
  const outcome = service.revoke(issued.id8);

  assert.equal(outcome.found, true);
  assert.equal(outcome.status, 'revoked');

  const result = service.verify(issued.key, { env: 'prod' });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'revoked');

  const record = service.store.getById(issued.id8);
  assert.equal(record.revoked, true);
  assert.equal(record.revokedAt, clock.nowIso());
});

test('rotating a key produces a successor that verifies and keeps the previous key usable', (t) => {
  const clock = createFixedClock('2026-11-18T00:00:00.000Z');
  const service = quietService({
    dataFile: temporaryDataFile(t),
    clock,
    graceMs: 60 * 60 * 1000,
  });

  const issued = service.issue({ ownerId: 'billing-team', scopes: ['invoices:read'] });
  const rotated = service.rotate(issued.id8);

  assert.notEqual(rotated.key, issued.key);
  assert.equal(rotated.ownerId, 'billing-team');
  assert.equal(rotated.previousId8, issued.id8);

  assert.equal(service.verify(rotated.key, { env: 'prod' }).ok, true);
  assert.equal(service.verify(issued.key, { env: 'prod' }).ok, true);
});

test('malformed and unknown keys are rejected', (t) => {
  const service = quietService({
    dataFile: temporaryDataFile(t),
    clock: createFixedClock('2026-11-18T00:00:00.000Z'),
  });

  assert.deepEqual(service.verify('not-a-key', { env: 'prod' }), {
    ok: false,
    reason: 'invalid_format',
  });

  assert.deepEqual(service.verify('kl_prod_0000_00', { env: 'prod' }), {
    ok: false,
    reason: 'invalid_format',
  });
});
