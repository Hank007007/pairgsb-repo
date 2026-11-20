import test from 'node:test';
import assert from 'node:assert/strict';

import { makeBundle } from './helpers.js';
import {
  selectEntry,
  effectiveHost,
  credentialHeader,
  injectIntoRequest,
} from '../src/inject.js';

const BEARER = {
  id: 'partner-bearer',
  host: 'api.partner.com',
  scheme: 'bearer',
  value: 'tok_partner_live',
};

const API_KEY = {
  id: 'billing-key',
  host: 'billing.example.net',
  scheme: 'api-key',
  value: 'key_billing_live',
};

test('effectiveHost omits the default port and keeps an explicit one', () => {
  assert.equal(effectiveHost(new URL('https://api.partner.com/v1')), 'api.partner.com');
  assert.equal(effectiveHost(new URL('https://api.partner.com:443/v1')), 'api.partner.com');
  assert.equal(effectiveHost(new URL('http://api.partner.com:80/v1')), 'api.partner.com');
  assert.equal(effectiveHost(new URL('https://api.partner.com:8443/v1')), 'api.partner.com:8443');
  assert.equal(effectiveHost(new URL('HTTPS://API.Partner.COM/v1')), 'api.partner.com');
});

test('selectEntry returns the bearer entry for its own host', () => {
  const bundle = makeBundle([BEARER]);
  const entry = selectEntry(bundle, 'https://api.partner.com/v1/orders');
  assert.equal(entry?.id, 'partner-bearer');
});

test('selectEntry returns null for an unrelated host', () => {
  const bundle = makeBundle([BEARER]);
  assert.equal(selectEntry(bundle, 'https://unrelated.example.org/v1'), null);
});

test('selectEntry prefers the entry that carries an explicit port', () => {
  const bundle = makeBundle([
    BEARER,
    { ...BEARER, id: 'partner-8443', host: 'api.partner.com:8443', value: 'tok_partner_8443' },
  ]);
  const entry = selectEntry(bundle, 'https://api.partner.com:8443/v1/orders');
  assert.equal(entry?.id, 'partner-8443');
});

test('credentialHeader renders both supported schemes', () => {
  assert.deepEqual(credentialHeader({ scheme: 'bearer', value: 'abc' }), {
    name: 'Authorization',
    value: 'Bearer abc',
  });
  assert.deepEqual(credentialHeader({ scheme: 'api-key', value: 'abc' }), {
    name: 'X-Api-Key',
    value: 'abc',
  });
});

test('injectIntoRequest adds the api-key header without touching caller headers', () => {
  const bundle = makeBundle([API_KEY]);
  const { entry, headers } = injectIntoRequest(bundle, 'https://billing.example.net/invoices', {
    Accept: 'application/json',
  });
  assert.equal(entry?.id, 'billing-key');
  assert.equal(headers['X-Api-Key'], 'key_billing_live');
  assert.equal(headers.Accept, 'application/json');
});

test('injectIntoRequest replaces a caller-supplied credential header regardless of casing', () => {
  const bundle = makeBundle([BEARER]);
  const { headers } = injectIntoRequest(bundle, 'https://api.partner.com/v1', {
    authorization: 'Bearer stale',
  });
  const keys = Object.keys(headers).map((key) => key.toLowerCase());
  assert.equal(keys.filter((key) => key === 'authorization').length, 1);
  assert.equal(headers.Authorization, 'Bearer tok_partner_live');
});

test('injectIntoRequest leaves the request untouched when nothing matches', () => {
  const bundle = makeBundle([BEARER]);
  const { entry, headers } = injectIntoRequest(bundle, 'https://unrelated.example.org/', { Accept: '*/*' });
  assert.equal(entry, null);
  assert.deepEqual(headers, { Accept: '*/*' });
});
