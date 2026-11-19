'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { canonicalHost, selectCredential, matchesNoProxy, parseNoProxy } = require('../src/routing');

const RULES = [
  { host: 'api.example.com', credential: 'api' },
  { host: 'example.com', credential: 'apex' },
  { host: 'internal.example.com', credential: 'internal' },
];

test('an exact rule host matches', () => {
  assert.equal(selectCredential(RULES, 'api.example.com').credential, 'api');
});

test('a subdomain of a rule host matches the rule', () => {
  assert.equal(selectCredential(RULES, 'eu.api.example.com').credential, 'api');
});

test('the most specific matching rule wins', () => {
  assert.equal(selectCredential(RULES, 'internal.example.com').credential, 'internal');
  assert.equal(selectCredential(RULES, 'other.example.com').credential, 'apex');
});

test('a host outside the rule set has no credential', () => {
  assert.equal(selectCredential(RULES, 'other.test'), null);
  assert.equal(selectCredential(RULES, 'example.net'), null);
  assert.equal(selectCredential(RULES, ''), null);
});

test('host comparison is case-insensitive', () => {
  assert.equal(selectCredential(RULES, 'API.Example.COM').credential, 'api');
});

test('canonicalHost removes brackets and a trailing dot', () => {
  assert.equal(canonicalHost('[::1]'), '::1');
  assert.equal(canonicalHost('Host.Example.'), 'host.example');
  assert.equal(canonicalHost('   '), null);
});

test('parseNoProxy splits on commas and whitespace', () => {
  assert.deepEqual(parseNoProxy({ NO_PROXY: 'a.test, b.test  c.test' }), ['a.test', 'b.test', 'c.test']);
  assert.deepEqual(parseNoProxy({}), []);
});

test('NO_PROXY matches an exact host and its subdomains', () => {
  const env = { NO_PROXY: 'corp.internal, *.svc.internal' };
  assert.equal(matchesNoProxy('corp.internal', 80, env), true);
  assert.equal(matchesNoProxy('db.corp.internal', 80, env), true);
  assert.equal(matchesNoProxy('api.svc.internal', 443, env), true);
  assert.equal(matchesNoProxy('api.example.com', 443, env), false);
});

test('a NO_PROXY entry with a port only matches that port', () => {
  const env = { NO_PROXY: 'corp.internal:8080' };
  assert.equal(matchesNoProxy('corp.internal', 8080, env), true);
  assert.equal(matchesNoProxy('corp.internal', 80, env), false);
});
