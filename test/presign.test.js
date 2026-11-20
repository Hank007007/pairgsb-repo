import test from 'node:test';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';

import { load } from '../src/keys.js';
import { presign } from '../src/presign.js';
import { verify } from '../src/verify.js';

const KEYRING_PATH = fileURLToPath(new URL('../keys.json', import.meta.url));
const keyring = load(KEYRING_PATH);

const NOW = 1_767_225_600;

function sign(overrides = {}) {
  return presign({
    bucket: 'assets',
    key: 'reports/2024/q1.pdf',
    tenant: 'demo',
    keyring,
    now: NOW,
    expiresIn: 600,
    ...overrides,
  });
}

test('issues a link carrying the protocol parameters', () => {
  const url = new URL(sign());
  assert.equal(url.pathname, '/assets/reports/2024/q1.pdf');
  assert.equal(url.searchParams.get('X-Seal-Alg'), 'HS256');
  assert.equal(url.searchParams.get('X-Seal-Method'), 'GET');
  assert.equal(url.searchParams.get('X-Seal-Expires'), String(NOW + 600));
  assert.equal(url.searchParams.get('X-Seal-Tenant'), 'demo');
  assert.equal(url.searchParams.get('X-Seal-Version'), '1');
  assert.match(url.searchParams.get('sig'), /^[0-9a-f]{64}$/);
});

test('round-trips a GET link with no caller query', () => {
  const result = verify(sign(), { keyring, now: NOW });
  assert.equal(result.ok, true);
  assert.equal(result.tenant, 'demo');
  assert.equal(result.method, 'GET');
  assert.equal(result.bucket, 'assets');
  assert.equal(result.key, 'reports/2024/q1.pdf');
  assert.equal(result.expires, NOW + 600);
});

test('round-trips a link with caller query parameters', () => {
  const url = sign({ method: 'PUT', query: { b: '2', a: '1' } });
  const result = verify(url, { keyring, now: NOW });
  assert.equal(result.ok, true);
  assert.equal(result.method, 'PUT');
});

test('round-trips caller query values that need encoding', () => {
  const url = sign({ query: { filename: 'quarterly report.pdf', tag: 'a/b' } });
  const result = verify(url, { keyring, now: NOW });
  assert.equal(result.ok, true);
});

test('round-trips a rotated tenant using its current version', () => {
  const url = presign({
    bucket: 'media',
    key: 'images/logo.png',
    tenant: 'acme',
    keyring,
    now: NOW,
  });
  const result = verify(url, { keyring, now: NOW });
  assert.equal(result.ok, true);
  assert.equal(result.tenant, 'acme');
  assert.equal(result.version, 2);
});

test('rejects a link whose signature was altered', () => {
  const url = new URL(sign());
  const sig = url.searchParams.get('sig');
  const flipped = (sig[0] === '0' ? '1' : '0') + sig.slice(1);
  url.searchParams.set('sig', flipped);
  const result = verify(url.toString(), { keyring, now: NOW });
  assert.equal(result.ok, false);
});

test('rejects a link whose expiry was altered', () => {
  const url = new URL(sign());
  const expires = Number(url.searchParams.get('X-Seal-Expires'));
  url.searchParams.set('X-Seal-Expires', String(expires + 3600));
  const result = verify(url.toString(), { keyring, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'bad-signature');
});

test('rejects a link with no signature', () => {
  const url = new URL(sign());
  url.searchParams.delete('sig');
  const result = verify(url.toString(), { keyring, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'missing-signature');
});

test('rejects a string that is not a URL', () => {
  const result = verify('not a url', { keyring, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'malformed-url');
});
