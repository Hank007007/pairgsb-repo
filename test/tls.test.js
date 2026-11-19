'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { agentOptions, resetTrustCache } = require('../src/tls');
const { loadVault, getCredential } = require('../src/vault');
const { fixturePath } = require('./helpers');

test('upstream connection parameters are bound to the hop host', () => {
  resetTrustCache();
  const params = agentOptions({ host: 'api.example.com', caFile: fixturePath('ca.pem') });
  assert.equal(params.servername, 'api.example.com');
  assert.equal(params.keepAlive, false);
  assert.ok(Array.isArray(params.ca));
  assert.ok(params.ca.length > 0);
});

test('the fleet CA file adds anchors', () => {
  resetTrustCache();
  const params = agentOptions({ host: 'api.example.com', caFile: fixturePath('ca.pem') });
  const joined = params.ca.join('\n');
  assert.match(joined, /BEGIN CERTIFICATE/);
});

test('a credential that declares a client certificate supplies cert and key', () => {
  const vault = loadVault(fixturePath('vault.json'));
  const credential = getCredential(vault, 'svc-example-mtls');
  const params = agentOptions({ host: 'mtls.example.com', credential, caFile: fixturePath('ca.pem') });
  assert.ok(Buffer.isBuffer(params.cert));
  assert.ok(Buffer.isBuffer(params.key));
  assert.match(params.cert.toString('utf8'), /BEGIN CERTIFICATE/);
  assert.match(params.key.toString('utf8'), /BEGIN PRIVATE KEY/);
});

test('a credential without client material offers no certificate', () => {
  const vault = loadVault(fixturePath('vault.json'));
  const credential = getCredential(vault, 'svc-example-api');
  const params = agentOptions({ host: 'api.example.com', credential, caFile: fixturePath('ca.pem') });
  assert.equal(params.cert, undefined);
  assert.equal(params.key, undefined);
});

test('no trust anchors are configured when no CA file is present', () => {
  resetTrustCache();
  const params = agentOptions({ host: 'api.example.com', caFile: null });
  assert.equal(params.ca, undefined);
});
