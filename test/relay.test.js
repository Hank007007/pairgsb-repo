import test from 'node:test';
import assert from 'node:assert/strict';

import { createRelay, compileBundle } from '../src/index.js';
import { makeBundle, createFakeTransport, reply } from './helpers.js';

const PARTNER = {
  id: 'partner-api',
  host: 'api.partner.com',
  scheme: 'bearer',
  value: 'tok_partner_live',
};

const BILLING = {
  id: 'billing-api',
  host: 'billing.example.net',
  scheme: 'api-key',
  value: 'key_billing_live',
};

test('request injects the bearer token for the routed host', async () => {
  const fake = createFakeTransport([reply(200, { body: '{"ok":true}' })]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  const result = await relay.request('https://api.partner.com/v1/orders');

  assert.equal(result.status, 200);
  assert.equal(result.body, '{"ok":true}');
  assert.equal(fake.callCount, 1);
  assert.equal(fake.last().headers.Authorization, 'Bearer tok_partner_live');
  relay.close();
});

test('request injects the api key for the routed host', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({ bundle: makeBundle([BILLING]), transport: fake.transport });

  await relay.request('https://billing.example.net/invoices', { method: 'POST', body: '{}' });

  assert.equal(fake.last().headers['X-Api-Key'], 'key_billing_live');
  assert.equal(fake.last().method, 'POST');
  relay.close();
});

test('request sends no credential to a host outside the bundle', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  const result = await relay.request('https://public.example.org/status');

  assert.equal(result.status, 200);
  assert.equal(fake.last().headers.Authorization, undefined);
  assert.equal(fake.last().headers['X-Api-Key'], undefined);
  relay.close();
});

test('request preserves caller headers alongside the injected one', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  await relay.request('https://api.partner.com/v1', {
    headers: { Accept: 'application/json', 'User-Agent': 'svc-a/2.0' },
  });

  assert.equal(fake.last().headers.Accept, 'application/json');
  assert.equal(fake.last().headers['User-Agent'], 'svc-a/2.0');
  assert.equal(fake.last().headers.Authorization, 'Bearer tok_partner_live');
  relay.close();
});

test('request follows a same-host redirect and keeps the credential', async () => {
  const fake = createFakeTransport([
    reply(302, { headers: { location: '/v2/orders' } }),
    reply(200, { body: 'moved' }),
  ]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  const result = await relay.request('https://api.partner.com/v1/orders');

  assert.equal(result.status, 200);
  assert.equal(result.redirects, 1);
  assert.equal(fake.callCount, 2);
  assert.equal(fake.sent[1].url, 'https://api.partner.com/v2/orders');
  assert.equal(fake.sent[1].headers.Authorization, 'Bearer tok_partner_live');
  relay.close();
});

test('request retries a retryable status and succeeds', async () => {
  const fake = createFakeTransport([reply(503), reply(200, { body: 'ok' })]);
  const relay = createRelay({
    bundle: makeBundle([PARTNER]),
    transport: fake.transport,
    baseDelayMs: 1,
    maxDelayMs: 2,
    random: () => 0.5,
  });

  const result = await relay.request('https://api.partner.com/v1/orders');

  assert.equal(result.status, 200);
  assert.equal(fake.callCount, 2);
  assert.equal(fake.sent[1].headers.Authorization, 'Bearer tok_partner_live');
  relay.close();
});

test('request gives up after maxAttempts retryable responses', async () => {
  const fake = createFakeTransport([reply(500), reply(500), reply(500), reply(200)]);
  const relay = createRelay({
    bundle: makeBundle([PARTNER]),
    transport: fake.transport,
    maxAttempts: 3,
    baseDelayMs: 1,
    maxDelayMs: 2,
    random: () => 0.5,
  });

  const result = await relay.request('https://api.partner.com/v1');
  assert.equal(result.status, 500);
  assert.equal(fake.callCount, 3);
  relay.close();
});

test('request does not retry a non-retryable 4xx', async () => {
  const fake = createFakeTransport([reply(404), reply(200)]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport, baseDelayMs: 1 });

  const result = await relay.request('https://api.partner.com/v1/missing');

  assert.equal(result.status, 404);
  assert.equal(fake.callCount, 1);
  relay.close();
});

test('a 303 redirect switches to GET without a body', async () => {
  const fake = createFakeTransport([
    reply(303, { headers: { Location: 'https://api.partner.com/v2/orders/42' } }),
    reply(200),
  ]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  await relay.request('https://api.partner.com/v1/orders', { method: 'POST', body: '{"qty":1}' });

  assert.equal(fake.sent[1].method, 'GET');
  assert.equal(fake.sent[1].body, null);
  relay.close();
});

test('a redirect loop surfaces as TOO_MANY_REDIRECTS', async () => {
  const fake = createFakeTransport([
    reply(302, { headers: { location: '/a' } }),
    reply(302, { headers: { location: '/b' } }),
    reply(302, { headers: { location: '/a' } }),
  ]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport, maxRedirects: 2 });

  await assert.rejects(relay.request('https://api.partner.com/start'), (error) => {
    assert.equal(error.code, 'TOO_MANY_REDIRECTS');
    return true;
  });
  relay.close();
});

test('a transport failure is retried and then surfaced', async () => {
  const fake = createFakeTransport(() => {
    throw new Error('socket hang up');
  });
  const relay = createRelay({
    bundle: makeBundle([PARTNER]),
    transport: fake.transport,
    maxAttempts: 2,
    baseDelayMs: 1,
    maxDelayMs: 2,
    random: () => 0.5,
  });

  await assert.rejects(relay.request('https://api.partner.com/v1'), (error) => {
    assert.equal(error.code, 'TRANSPORT_ERROR');
    return true;
  });
  assert.equal(fake.callCount, 2);
  relay.close();
});

test('an already-aborted signal rejects without sending', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });
  const controller = new AbortController();
  controller.abort();

  await assert.rejects(relay.request('https://api.partner.com/v1', { signal: controller.signal }), (error) => {
    assert.equal(error.code, 'ABORTED');
    return true;
  });
  assert.equal(fake.callCount, 0);
});

test('audit records capture the attempt metadata', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  await relay.request('https://api.partner.com/v1');

  const [record] = relay.audit.records();
  assert.equal(record.type, 'attempt');
  assert.equal(record.id, 'partner-api');
  assert.equal(record.method, 'GET');
  assert.equal(record.url, 'https://api.partner.com/v1');
  assert.equal(record.status, 200);
  assert.equal(record.attempt, 1);
  relay.close();
});

test('repeated requests carry distinct sequence numbers', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  await relay.request('https://api.partner.com/v1');
  await relay.request('https://api.partner.com/v2');

  const seqs = relay.audit.records().map((record) => record.seq);
  assert.deepEqual(seqs, [1, 2]);
  relay.close();
});

test('replacing the bundle changes what the next request injects', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  await relay.request('https://api.partner.com/v1');
  relay.setBundle(
    compileBundle({ secrets: [{ ...PARTNER, id: 'partner-api-v2', value: 'tok_partner_rotated' }] }, 99),
  );
  await relay.request('https://api.partner.com/v1');

  assert.equal(fake.sent[0].headers.Authorization, 'Bearer tok_partner_live');
  assert.equal(fake.sent[1].headers.Authorization, 'Bearer tok_partner_rotated');
  relay.close();
});

test('diagnostics reports the active revision and request counters', async () => {
  const fake = createFakeTransport([reply(200)]);
  const bundle = makeBundle([PARTNER, BILLING]);
  const relay = createRelay({ bundle, transport: fake.transport });

  await relay.request('https://api.partner.com/v1');

  const diagnostics = relay.diagnostics();
  assert.equal(diagnostics.revision, bundle.revision);
  assert.equal(diagnostics.entries, 2);
  assert.equal(diagnostics.requests, 1);
  assert.equal(diagnostics.attempts, 1);
  relay.close();
});

test('invalid options reject with INVALID_OPTIONS', async () => {
  const relay = createRelay({ transport: createFakeTransport([reply(200)]).transport });
  await assert.rejects(relay.request(''), (error) => {
    assert.equal(error.code, 'INVALID_OPTIONS');
    return true;
  });
  relay.close();
});
