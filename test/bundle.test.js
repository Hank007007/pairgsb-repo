import test from 'node:test';
import assert from 'node:assert/strict';

import { compileBundle, parseBundle, normalizeHost, loadBundle, createBundleStore } from '../src/bundle.js';
import { BundleError } from '../src/errors.js';

const SAMPLE = {
  version: 1,
  secrets: [
    {
      id: 'partner-api',
      host: 'api.partner.com',
      scheme: 'bearer',
      value: 'tok_live_123',
      not_before: '2026-01-01T00:00:00Z',
      expires_at: '2026-12-31T23:59:59Z',
    },
    { id: 'plain', host: 'plain.example.net', scheme: 'api-key', value: 'k_plain' },
  ],
};

test('compileBundle stores every entry with parsed instants', () => {
  const bundle = compileBundle(SAMPLE, 1, '<memory>');
  assert.equal(bundle.revision, 1);
  assert.equal(bundle.entries.length, 2);
  const [partner] = bundle.entries;
  assert.equal(partner.id, 'partner-api');
  assert.equal(partner.host, 'api.partner.com');
  assert.equal(partner.scheme, 'bearer');
  assert.ok(partner.notBefore instanceof Date);
  assert.ok(partner.expiresAt instanceof Date);
  assert.equal(partner.notBefore.toISOString(), '2026-01-01T00:00:00.000Z');
  assert.equal(partner.expiresAt.toISOString(), '2026-12-31T23:59:59.000Z');
});

test('compileBundle defaults missing bounds to null', () => {
  const bundle = compileBundle({ secrets: [SAMPLE.secrets[1]] }, 2);
  assert.equal(bundle.entries[0].notBefore, null);
  assert.equal(bundle.entries[0].expiresAt, null);
});

test('compileBundle result is frozen', () => {
  const bundle = compileBundle(SAMPLE, 1);
  assert.ok(Object.isFrozen(bundle));
  assert.ok(Object.isFrozen(bundle.entries));
  assert.ok(Object.isFrozen(bundle.entries[0]));
});

test('normalizeHost lower-cases, keeps explicit ports and drops a root dot', () => {
  assert.equal(normalizeHost('API.Partner.COM'), 'api.partner.com');
  assert.equal(normalizeHost('api.partner.com.'), 'api.partner.com');
  assert.equal(normalizeHost('api.partner.com:8443'), 'api.partner.com:8443');
  assert.equal(normalizeHost('[2001:DB8::1]:8443'), '[2001:db8::1]:8443');
});

test('compileBundle rejects duplicate ids', () => {
  const bad = { secrets: [SAMPLE.secrets[1], { ...SAMPLE.secrets[1], host: 'other.example.net' }] };
  assert.throws(() => compileBundle(bad, 1), (error) => error instanceof BundleError && error.code === 'BUNDLE_INVALID');
});

test('compileBundle rejects an unsupported scheme', () => {
  const bad = { secrets: [{ id: 'x', host: 'x.example', scheme: 'basic', value: 'v' }] };
  assert.throws(() => compileBundle(bad, 1), BundleError);
});

test('parseBundle reports malformed JSON as a bundle error', () => {
  assert.throws(() => parseBundle('{ not json', 1), (error) => {
    assert.ok(error instanceof BundleError);
    assert.equal(error.code, 'BUNDLE_INVALID');
    return true;
  });
});

test('loadBundle reads through an injected reader', async () => {
  const bundle = await loadBundle('/virtual/bundle.json', 7, async () => JSON.stringify(SAMPLE));
  assert.equal(bundle.revision, 7);
  assert.equal(bundle.source, '/virtual/bundle.json');
  assert.equal(bundle.entries.length, 2);
});

test('bundle store swaps revisions atomically and notifies subscribers', () => {
  const first = compileBundle(SAMPLE, 1);
  const second = compileBundle({ secrets: [SAMPLE.secrets[1]] }, 2);
  const store = createBundleStore(first);
  const seen = [];
  const unsubscribe = store.subscribe((next, previous) => {
    seen.push([previous.revision, next.revision]);
  });

  assert.equal(store.current, first);
  store.replace(second);
  assert.equal(store.current, second);
  assert.deepEqual(seen, [[1, 2]]);

  unsubscribe();
  store.replace(first);
  assert.equal(seen.length, 1);
});

test('bundle store survives a throwing subscriber', () => {
  const first = compileBundle(SAMPLE, 1);
  const second = compileBundle(SAMPLE, 2);
  const store = createBundleStore(first);
  store.subscribe(() => {
    throw new Error('subscriber boom');
  });
  assert.doesNotThrow(() => store.replace(second));
  assert.equal(store.current.revision, 2);
});
