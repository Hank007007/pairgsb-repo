import test from 'node:test';
import assert from 'node:assert/strict';

import { parseKeyring, activeVersion, candidateVersions } from '../src/keys.js';

const DOC = {
  global: [{ version: 1, secret: 'global-secret' }],
  demo: [{ version: 1, secret: 'demo-secret' }],
  acme: [
    { version: 2, secret: 'acme-v2' },
    { version: 1, secret: 'acme-v1', not_after: 1767225600 },
  ],
};

const text = JSON.stringify(DOC);

test('loads every tenant from the keyring document', () => {
  const keyring = parseKeyring(text);
  assert.deepEqual(Object.keys(keyring).sort(), ['acme', 'demo', 'global']);
  assert.equal(keyring.demo.length, 1);
  assert.equal(keyring.demo[0].secret, 'demo-secret');
});

test('reads version numbers as integers', () => {
  const keyring = parseKeyring(text);
  assert.equal(keyring.acme[0].version, 2);
  assert.equal(keyring.acme[1].not_after, 1767225600);
});

test('active version is the first entry', () => {
  const keyring = parseKeyring(text);
  assert.equal(activeVersion(keyring.acme).version, 2);
});

test('candidate versions include the active version', () => {
  const keyring = parseKeyring(text);
  const candidates = candidateVersions(keyring.demo);
  assert.equal(candidates.length, 1);
  assert.equal(candidates[0].secret, 'demo-secret');
});

test('rejects documents that are not objects', () => {
  assert.throws(() => parseKeyring('[]'), /top level/);
  assert.throws(() => parseKeyring('nope'), /not valid JSON/);
});

test('rejects a tenant with no versions', () => {
  assert.throws(() => parseKeyring(JSON.stringify({ demo: [] })), /at least one version/);
});

test('rejects a version with a bad secret', () => {
  assert.throws(
    () => parseKeyring(JSON.stringify({ demo: [{ version: 1 }] })),
    /secret/,
  );
});
