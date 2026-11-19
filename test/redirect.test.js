'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { follow, resolveLocation, isRedirect, preservesBody, RedirectError } = require('../src/redirect');

const HEADERS = new Map([
  ['accept', { name: 'Accept', value: 'application/json' }],
  ['authorization', { name: 'Authorization', value: 'Bearer fixture' }],
]);

function hopFor(url) {
  const parsed = new URL(url);
  return {
    url,
    host: parsed.hostname,
    port: Number(parsed.port || 80),
    protocol: parsed.protocol,
    path: `${parsed.pathname}${parsed.search}`,
    method: 'GET',
    headers: new Map(HEADERS),
    body: null,
    hop: 0,
  };
}

test('only 301, 302, 303, 307 and 308 are followed', () => {
  for (const status of [301, 302, 303, 307, 308]) assert.equal(isRedirect(status), true);
  for (const status of [200, 204, 300, 304, 400, 500]) assert.equal(isRedirect(status), false);
});

test('307 and 308 preserve the body, the others do not', () => {
  assert.equal(preservesBody(307), true);
  assert.equal(preservesBody(308), true);
  assert.equal(preservesBody(301), false);
  assert.equal(preservesBody(302), false);
  assert.equal(preservesBody(303), false);
});

test('resolveLocation handles absolute, scheme-relative and path forms', () => {
  assert.equal(resolveLocation('http://a.test/x/y', '/z').href, 'http://a.test/z');
  assert.equal(resolveLocation('http://a.test/x/y', '//b.test/z').href, 'http://b.test/z');
  assert.equal(resolveLocation('http://a.test/x/y', 'z').href, 'http://a.test/x/z');
  assert.equal(resolveLocation('http://a.test/x/y', 'https://b.test/z').href, 'https://b.test/z');
});

test('resolveLocation refuses non-http schemes and embedded credentials', () => {
  assert.throws(() => resolveLocation('http://a.test/', 'file:///etc/passwd'), RedirectError);
  assert.throws(() => resolveLocation('http://a.test/', 'ftp://b.test/x'), RedirectError);
  assert.throws(() => resolveLocation('http://a.test/', 'http://user:pass@b.test/'), RedirectError);
});

test('follow builds the next hop with the resolved target', () => {
  const previous = hopFor('http://a.test/start');
  const next = follow(previous, { statusCode: 302, headers: { location: '/next?x=1' } });
  assert.equal(next.host, 'a.test');
  assert.equal(next.port, 80);
  assert.equal(next.path, '/next?x=1');
  assert.equal(next.method, 'GET');
  assert.equal(next.hop, 1);
});

test('follow downgrades 303 to a bodyless GET', () => {
  const previous = hopFor('http://a.test/start');
  previous.method = 'POST';
  previous.body = Buffer.from('payload');
  previous.headers.set('content-type', { name: 'Content-Type', value: 'application/json' });
  const next = follow(previous, { statusCode: 303, headers: { location: '/done' } });
  assert.equal(next.method, 'GET');
  assert.equal(next.body, null);
  assert.equal(next.headers.has('content-type'), false);
});

test('follow keeps method and body for 307', () => {
  const previous = hopFor('http://a.test/start');
  previous.method = 'POST';
  previous.body = Buffer.from('payload');
  const next = follow(previous, { statusCode: 307, headers: { location: '/keep' } });
  assert.equal(next.method, 'POST');
  assert.deepEqual(next.body, Buffer.from('payload'));
});
