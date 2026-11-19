'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const { createAuditLog, redactQuery } = require('../src/audit');
const { loadVault, getCredential } = require('../src/vault');
const { fixturePath, makeTempDir } = require('./helpers');

test('the query string is reduced to its first key name', () => {
  assert.equal(redactQuery('/v1/items?token=abc&page=2'), '/v1/items?token=…');
  assert.equal(redactQuery('/v1/items?token=abc'), '/v1/items?token=…');
  assert.equal(redactQuery('/v1/items'), '/v1/items');
  assert.equal(redactQuery('/v1/items#frag'), '/v1/items');
  assert.equal(redactQuery(''), '/');
});

test('one record is appended per hop with the documented fields', () => {
  const dir = makeTempDir();
  const file = path.join(dir, 'audit.jsonl');
  const audit = createAuditLog({ file });

  audit.logRequest({
    ts: new Date('2026-01-02T03:04:05.000Z'),
    host: 'api.example.com',
    path: '/v1/items?token=abc',
    status: 200,
    durationMs: 12.3456,
    keyId: 'svc-example-api',
    error: null,
  });
  audit.logRequest({
    ts: new Date('2026-01-02T03:04:06.000Z'),
    host: 'api.example.com',
    path: '/v1/items',
    status: null,
    durationMs: 3,
    keyId: 'svc-example-api',
    error: Object.assign(new Error('boom'), { code: 'ETIMEOUT' }),
  });
  audit.close();

  const lines = fs.readFileSync(file, 'utf8').trim().split('\n');
  assert.equal(lines.length, 2);

  const first = JSON.parse(lines[0]);
  assert.equal(first.ts, '2026-01-02T03:04:05.000Z');
  assert.equal(first.host, 'api.example.com');
  assert.equal(first.path, '/v1/items?token=…');
  assert.equal(first.status, 200);
  assert.equal(first.durationMs, 12.346);
  assert.equal(first.keyId, 'svc-example-api');
  assert.equal(first.error, null);

  const second = JSON.parse(lines[1]);
  assert.equal(second.status, null);
  assert.equal(second.error, 'ETIMEOUT');
});

test('the audit file is created owner-only', { skip: process.platform === 'win32' }, () => {
  const dir = makeTempDir();
  const file = path.join(dir, 'audit.jsonl');
  const audit = createAuditLog({ file });
  audit.logRequest({ host: 'a.test', path: '/', status: 200, durationMs: 1, keyId: null });
  audit.close();
  const mode = fs.statSync(file).mode & 0o777;
  assert.equal(mode, 0o600);
});

test('the credential key id recorded for a hop is the vault key, not the secret', () => {
  const vault = loadVault(fixturePath('vault.json'));
  const credential = getCredential(vault, 'svc-example-api');
  assert.equal(credential.keyId, 'svc-example-api');
  assert.notEqual(credential.keyId, credential.secret);
});
