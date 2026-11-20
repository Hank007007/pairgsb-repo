import test from 'node:test';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';

import { canonicalQuery } from '../src/canonical.js';
import { load, parseKeyring } from '../src/keys.js';
import {
  PROTOCOL_ALG,
  PROTOCOL_EXPIRES,
  PROTOCOL_METHOD,
  PROTOCOL_TENANT,
  PROTOCOL_VERSION,
  SIG_PARAM,
  buildCanonicalString,
  presign,
} from '../src/presign.js';
import { hmac, signaturesMatch } from '../src/sign.js';
import { verify } from '../src/verify.js';

const KEYRING_PATH = fileURLToPath(new URL('../keys.json', import.meta.url));
const keyring = load(KEYRING_PATH);
const NOW = 1_767_225_600;

function sign(overrides = {}) {
  return presign({
    bucket: 'assets',
    key: 'a.txt',
    tenant: 'demo',
    keyring,
    now: NOW,
    ...overrides,
  });
}

function tamper(url, change) {
  const parsed = new URL(url);
  change(parsed);
  return parsed.toString();
}

test('X-Seal-Method is bound by the signature', () => {
  const methodUrl = tamper(sign(), (url) => {
    url.searchParams.set(PROTOCOL_METHOD, 'PUT');
  });
  const result = verify(methodUrl, { keyring, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'bad-signature');
});

test('unsupported X-Seal-Alg values are rejected without downgrading', () => {
  for (const alg of ['none', 'HS999', 'md5']) {
    const tampered = tamper(sign(), (url) => {
      url.searchParams.set(PROTOCOL_ALG, alg);
    });
    const result = verify(tampered, { keyring, now: NOW });
    assert.equal(result.ok, false, alg);
    assert.equal(result.reason, 'unsupported-alg', alg);
  }
});

test('expiry is checked against the verifier clock with the documented skew', () => {
  const url = sign();
  const expires = Number(new URL(url).searchParams.get(PROTOCOL_EXPIRES));

  assert.equal(verify(url, { keyring, now: NOW + 86400 }).reason, 'expired');
  assert.equal(verify(url, { keyring, now: expires + 30 }).ok, true);
  assert.equal(verify(url, { keyring, now: expires + 31 }).reason, 'expired');
});

test('a tenant absent from the keyring cannot sign or verify', () => {
  assert.throws(
    () => sign({ tenant: 'ghost' }),
    /no signing key available for tenant "ghost"/,
  );

  const tampered = tamper(sign(), (url) => {
    url.searchParams.set(PROTOCOL_TENANT, 'ghost');
  });
  const result = verify(tampered, { keyring, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'unknown-tenant');
});

test('caller query duplicates are preserved and cannot be prepended', () => {
  const url = sign({ query: 'tag=b' });
  const prepended = url.replace('tag=b', 'tag=a&tag=b');
  const result = verify(prepended, { keyring, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'bad-signature');
});

test('query encoding cannot smuggle a different parameter structure', () => {
  const url = sign({ query: 'a=1&b=2' });
  const encoded = url.replace('a=1&b=2', 'a=1%26b%3D2');
  const result = verify(encoded, { keyring, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'bad-signature');

  assert.equal(canonicalQuery('b=2&a=%31'), 'a=1&b=2');
  assert.equal(canonicalQuery('tag=a+b'), 'tag=a%20b');
  assert.notEqual(canonicalQuery('tag=b'), canonicalQuery('tag=a&tag=b'));
  assert.notEqual(canonicalQuery('a=1&b=2'), canonicalQuery('a=1%26b%3D2'));
});

test('duplicate protocol parameters are rejected', () => {
  const duplicated = tamper(sign(), (url) => {
    url.searchParams.append(PROTOCOL_METHOD, 'PUT');
  });
  const result = verify(duplicated, { keyring, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'duplicate-parameter');
});

test('invalid object keys are rejected instead of normalized', () => {
  for (const key of ['a/../b', 'a//b', 'a/b/', '/a', 'a/./b']) {
    assert.throws(() => sign({ key }), /invalid object key/, key);
  }

  const valid = sign({ key: 'b' });
  const parsed = new URL(valid);
  for (const pathname of [
    '/assets/a/../b',
    '/assets/a//b',
    '/assets/a/b/',
    '/assets/%2e%2e/b',
    '/assets/a%2F..%2Fb',
  ]) {
    const target = `${parsed.origin}${pathname}${parsed.search}`;
    const result = verify(target, { keyring, now: NOW });
    assert.equal(result.ok, false, pathname);
    assert.equal(result.reason, 'invalid-key', pathname);
  }
});

test('rotated keys verify until their not_after and no later', () => {
  const oldAcme = {
    acme: [{ version: 1, secret: 'acme-v1-4b8e2f', not_after: NOW }],
  };
  const oldLink = presign({
    bucket: 'assets',
    key: 'a.txt',
    tenant: 'acme',
    keyring: oldAcme,
    now: NOW - 1,
  });

  const before = verify(oldLink, { keyring, now: NOW - 1 });
  assert.equal(before.ok, true);
  assert.equal(before.version, 1);

  const atBoundary = verify(oldLink, { keyring, now: NOW });
  assert.equal(atBoundary.ok, true);
  assert.equal(atBoundary.version, 1);

  const after = verify(oldLink, { keyring, now: NOW + 1 });
  assert.equal(after.ok, false);
  assert.equal(after.reason, 'bad-signature');
});

test('an expired active version cannot sign and its signatures cannot verify', () => {
  assert.throws(
    () => sign({ tenant: 'legacy' }),
    /signing key for tenant "legacy" has expired/,
  );

  const canonical = buildCanonicalString({
    bucket: 'assets',
    key: 'a.txt',
    method: 'GET',
    query: [],
    expires: NOW + 600,
    tenant: 'legacy',
  });
  const url = new URL('https://storage.example/assets/a.txt');
  url.searchParams.set(PROTOCOL_ALG, 'HS256');
  url.searchParams.set(PROTOCOL_METHOD, 'GET');
  url.searchParams.set(PROTOCOL_EXPIRES, String(NOW + 600));
  url.searchParams.set(PROTOCOL_TENANT, 'legacy');
  url.searchParams.set(PROTOCOL_VERSION, '9');
  url.searchParams.set(SIG_PARAM, hmac('legacy-retired-5a5a', canonical));

  const result = verify(url.toString(), { keyring, now: NOW });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'bad-signature');
});

test('duplicate versions under one tenant are a load-time error', () => {
  const doc = JSON.stringify({
    demo: [
      { version: 1, secret: 'first' },
      { version: 1, secret: 'second' },
    ],
  });
  assert.throws(() => parseKeyring(doc), /duplicate version 1/);
});

test('signature comparison does not short-circuit at the first differing byte', () => {
  const expected = 'a'.repeat(64);
  const firstByteDiffers = `b${'a'.repeat(63)}`;
  const lastByteDiffers = `${'a'.repeat(63)}b`;

  assert.equal(signaturesMatch(expected, firstByteDiffers), false);
  assert.equal(signaturesMatch(expected, lastByteDiffers), false);
  assert.equal(signaturesMatch(expected, expected.slice(1)), false);

  for (let i = 0; i < 10_000; i += 1) {
    signaturesMatch(expected, firstByteDiffers);
    signaturesMatch(expected, lastByteDiffers);
  }

  const measure = (presented) => {
    const started = process.hrtime.bigint();
    for (let i = 0; i < 120_000; i += 1) signaturesMatch(expected, presented);
    return Number(process.hrtime.bigint() - started);
  };

  const firstMs = Math.min(measure(firstByteDiffers), measure(firstByteDiffers), measure(firstByteDiffers));
  const lastMs = Math.min(measure(lastByteDiffers), measure(lastByteDiffers), measure(lastByteDiffers));
  const ratio = firstMs / lastMs;
  assert.ok(ratio > 0.2 && ratio < 5, `unexpected timing ratio ${ratio}`);
});
