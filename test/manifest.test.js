'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');

const {
  buildManifest,
  canonicalJson,
  digestPayload,
  serializeManifest,
  signManifest,
  signingInput,
} = require('../src/protocol/manifest');
const { verifyManifest, verifyPayload } = require('../src/collector/verify');

const SECRET = 'test-secret';
const ORIGIN = 'web-01#a';

function makeManifest(overrides = {}) {
  return buildManifest({
    origin: ORIGIN,
    epoch: 0,
    firstSeq: 0,
    count: 2,
    digest: digestPayload('{"n":1}\n{"n":2}\n'),
    keyId: 'edge-a',
    ...overrides,
  });
}

test('canonicalJson sorts keys and escapes non-ASCII', () => {
  assert.equal(canonicalJson({ b: 1, a: 'x' }), '{"a":"x","b":1}');
  assert.equal(canonicalJson('café'), '"caf\\u00e9"');
  assert.equal(canonicalJson([1, true, null]), '[1,true,null]');
});

test('signing input omits the signature member', () => {
  const manifest = makeManifest();
  const signed = signManifest(manifest, SECRET);
  assert.equal(signingInput(signed), signingInput(manifest));
  assert.deepEqual(JSON.parse(canonicalJson(signed)), signed);
});

test('verifyManifest accepts a well-formed signed manifest', () => {
  const signed = signManifest(makeManifest(), SECRET);
  const raw = serializeManifest(signed);
  const verified = verifyManifest(raw, { keyring: { 'edge-a': { secret: SECRET } } });
  assert.equal(verified.origin, ORIGIN);
  assert.equal(verified.count, 2);
});

test('verifyPayload accepts a payload matching count and digest', () => {
  const body = '{"n":1}\n{"n":2}\n';
  const manifest = makeManifest({ digest: digestPayload(body) });
  const records = verifyPayload(manifest, Buffer.from(body, 'utf8'));
  assert.deepEqual(records, ['{"n":1}', '{"n":2}']);
});

test('verifyPayload reports a digest mismatch for altered payload bytes', () => {
  const manifest = makeManifest({ digest: digestPayload('{"n":1}\n{"n":2}\n') });
  assert.throws(
    () => verifyPayload(manifest, Buffer.from('{"n":1}\n{"n":3}\n', 'utf8')),
    (err) => err.code === 'digest_mismatch' && err.field === 'digest',
  );
});

test('posting the same manifest twice verifies the same way', () => {
  const signed = signManifest(makeManifest(), SECRET);
  const raw = serializeManifest(signed);
  const keyring = { 'edge-a': { secret: SECRET } };
  const first = verifyManifest(raw, { keyring });
  const second = verifyManifest(raw, { keyring });
  assert.deepEqual(first, second);
});
