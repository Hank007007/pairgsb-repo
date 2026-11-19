'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { loadVault, getCredential, injectionHeaders, normalizeHost, VaultError } = require('../src/vault');
const { fixturePath } = require('./helpers');

test('loadVault reads the fixture vault and its rules', () => {
  const vault = loadVault(fixturePath('vault.json'));
  assert.equal(vault.rules.length, 3);
  assert.deepEqual(
    vault.rules.map((rule) => rule.host).sort(),
    ['api.example.com', 'internal.example.com', 'localhost']
  );
  const credential = getCredential(vault, 'svc-example-api');
  assert.equal(credential.header, 'Authorization');
  assert.equal(credential.scheme, 'Bearer');
});

test('injectionHeaders builds the authentication header plus declared extras', () => {
  const vault = loadVault(fixturePath('vault.json'));
  const headers = injectionHeaders(getCredential(vault, 'svc-example-api'));
  const auth = headers.get('authorization');
  assert.ok(auth);
  assert.equal(auth.name, 'Authorization');
  assert.equal(auth.value, 'Bearer fixture-api-token-not-a-real-secret');
  assert.equal(headers.get('x-egress-tenant').value, 'example');
});

test('a credential with a custom header carries the secret on its own', () => {
  const vault = loadVault(fixturePath('vault.json'));
  const headers = injectionHeaders(getCredential(vault, 'svc-example-internal'));
  assert.equal(headers.get('x-api-key').value, 'fixture-internal-key-not-a-real-secret');
  assert.equal(headers.has('authorization'), false);
});

test('client certificate material is resolved relative to the vault file', () => {
  const vault = loadVault(fixturePath('vault.json'));
  const credential = getCredential(vault, 'svc-example-mtls');
  assert.ok(credential.clientCert);
  assert.equal(credential.clientCert.certFile, fixturePath('client.pem'));
  assert.equal(credential.clientCert.keyFile, fixturePath('client-key.pem'));
});

test('a vault with a duplicate rule host is rejected', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'egressvault-vault-'));
  const file = path.join(dir, 'vault.json');
  fs.writeFileSync(
    file,
    JSON.stringify({
      credentials: { key: { secret: 'x' } },
      rules: [
        { host: 'a.example.com', credential: 'key' },
        { host: 'a.example.com', credential: 'key' },
      ],
    })
  );
  assert.throws(() => loadVault(file), VaultError);
});

test('a rule for an unknown credential is rejected', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'egressvault-vault-'));
  const file = path.join(dir, 'vault.json');
  fs.writeFileSync(
    file,
    JSON.stringify({ credentials: { key: { secret: 'x' } }, rules: [{ host: 'a.example.com', credential: 'nope' }] })
  );
  assert.throws(() => loadVault(file), VaultError);
});

test('normalizeHost lower-cases and strips a trailing dot', () => {
  assert.equal(normalizeHost('API.Example.COM.'), 'api.example.com');
});
