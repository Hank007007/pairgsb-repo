import test from 'node:test';
import assert from 'node:assert/strict';

import {
  isRedirectStatus,
  resolveLocation,
  switchesToGet,
  followRedirect,
  REDIRECT_STATUS,
} from '../src/redirect.js';
import { InsecureRedirectError } from '../src/errors.js';

test('isRedirectStatus recognizes the supported statuses', () => {
  for (const status of REDIRECT_STATUS) {
    assert.equal(isRedirectStatus(status), true);
  }
  assert.equal(isRedirectStatus(200), false);
  assert.equal(isRedirectStatus(304), false);
});

test('resolveLocation resolves relative targets against the current URL', () => {
  assert.equal(
    resolveLocation('/v2/orders', 'https://api.partner.com/v1/orders').toString(),
    'https://api.partner.com/v2/orders',
  );
  assert.equal(
    resolveLocation('https://other.example.net/v1', 'https://api.partner.com/v1').toString(),
    'https://other.example.net/v1',
  );
});

test('switchesToGet covers 303 and POST-on-301/302', () => {
  assert.equal(switchesToGet(303, 'PUT'), true);
  assert.equal(switchesToGet(301, 'POST'), true);
  assert.equal(switchesToGet(302, 'POST'), true);
  assert.equal(switchesToGet(301, 'GET'), false);
  assert.equal(switchesToGet(307, 'POST'), false);
  assert.equal(switchesToGet(308, 'POST'), false);
});

test('followRedirect rewrites a POST to GET on 303 and drops the body', () => {
  const next = followRedirect(
    {
      url: 'https://api.partner.com/v1/orders',
      method: 'POST',
      body: '{"qty":1}',
      headers: { Accept: 'application/json' },
    },
    303,
    '/v1/orders/42',
  );
  assert.equal(next.url, 'https://api.partner.com/v1/orders/42');
  assert.equal(next.method, 'GET');
  assert.equal(next.body, null);
  assert.equal(next.redirectedFrom, 'https://api.partner.com/v1/orders');
});

test('followRedirect preserves method and body on 307', () => {
  const next = followRedirect(
    { url: 'https://api.partner.com/v1', method: 'POST', body: 'payload', headers: {} },
    307,
    '/v2',
  );
  assert.equal(next.method, 'POST');
  assert.equal(next.body, 'payload');
  assert.equal(next.url, 'https://api.partner.com/v2');
});

test('followRedirect carries caller headers to the next hop', () => {
  const next = followRedirect(
    { url: 'https://api.partner.com/v1', method: 'GET', body: null, headers: { Accept: 'application/json' } },
    302,
    '/v2',
  );
  assert.equal(next.headers.Accept, 'application/json');
});

test('followRedirect refuses an https-to-http downgrade', () => {
  assert.throws(
    () =>
      followRedirect(
        { url: 'https://api.partner.com/v1', method: 'GET', body: null, headers: {} },
        302,
        'http://api.partner.com/v1',
      ),
    (error) => error instanceof InsecureRedirectError && error.code === 'INSECURE_REDIRECT',
  );
});

test('followRedirect permits the downgrade only when explicitly allowed', () => {
  const next = followRedirect(
    { url: 'https://api.partner.com/v1', method: 'GET', body: null, headers: {} },
    302,
    'http://api.partner.com/v1',
    { allowInsecureRedirects: true },
  );
  assert.equal(next.url, 'http://api.partner.com/v1');
});
