import test from 'node:test';
import assert from 'node:assert/strict';

import { createRelay, compileBundle } from '../src/index.js';
import { REDACTED, redactHeaders, redactUrl } from '../src/audit.js';
import { selectEntry } from '../src/inject.js';
import { makeBundle, createFakeTransport, reply } from './helpers.js';

const PARTNER = {
  id: 'partner-api',
  host: 'api.partner.com',
  scheme: 'bearer',
  value: 'tok_partner_live',
};

test('host selection is exact and never uses substring, suffix or wrong-port matches', () => {
  const bundle = makeBundle([PARTNER]);

  assert.equal(selectEntry(bundle, 'https://api.partner.com/v1')?.id, 'partner-api');
  assert.equal(selectEntry(bundle, 'https://api.partner.com./v1')?.id, 'partner-api');
  assert.equal(selectEntry(bundle, 'https://api.partner.com:443/v1')?.id, 'partner-api');

  assert.equal(selectEntry(bundle, 'https://api.partner.com.attacker.example/v1'), null);
  assert.equal(selectEntry(bundle, 'https://evil-api.partner.com/v1'), null);
  assert.equal(selectEntry(bundle, 'https://api.partner.com:8443/v1'), null);
});

test('request sends no credential to lookalike hosts or a non-default port', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  await relay.request('https://api.partner.com.attacker.example/v1');
  await relay.request('https://api.partner.com:8443/v1');
  await relay.request('https://evil-api.partner.com/v1');

  for (const sent of fake.sent) {
    assert.equal(sent.headers.Authorization, undefined);
  }
  relay.close();
});

test('a cross-host redirect is routed from scratch and carries no source credential', async () => {
  const fake = createFakeTransport([
    reply(302, { headers: { location: 'https://collector.attacker.example/x' } }),
    reply(200),
  ]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  const result = await relay.request('https://api.partner.com/v1', {
    headers: { Accept: 'application/json', authorization: 'Bearer caller-supplied' },
  });

  assert.equal(result.redirects, 1);
  assert.equal(fake.sent[0].headers.Authorization, 'Bearer tok_partner_live');
  assert.equal(fake.sent[1].url, 'https://collector.attacker.example/x');
  assert.equal(fake.sent[1].headers.Authorization, undefined);
  assert.equal(fake.sent[1].headers.authorization, undefined);
  assert.equal(fake.sent[1].headers.Accept, 'application/json');
  relay.close();
});

test('a cross-host redirect injects the target host credential, not the source one', async () => {
  const billing = {
    id: 'billing-api',
    host: 'billing.example.net',
    scheme: 'api-key',
    value: 'key_billing_live',
  };
  const fake = createFakeTransport([
    reply(302, { headers: { location: 'https://billing.example.net/invoices' } }),
    reply(200),
  ]);
  const relay = createRelay({ bundle: makeBundle([PARTNER, billing]), transport: fake.transport });

  await relay.request('https://api.partner.com/v1');

  assert.equal(fake.sent[1].headers.Authorization, undefined);
  assert.equal(fake.sent[1].headers['X-Api-Key'], 'key_billing_live');
  relay.close();
});

test('audit redaction is case-insensitive and scrubs bundle values outside auth headers', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({ bundle: makeBundle([PARTNER]), transport: fake.transport });

  const result = await relay.request(
    'https://api.partner.com/v1?api_key=tok_partner_live&page=2',
    { headers: { 'X-Custom': 'prefix tok_partner_live suffix' } },
  );

  const [record] = relay.audit.records();
  assert.equal(record.headers.Authorization, REDACTED);
  assert.ok(record.headers['X-Custom'].includes(REDACTED));
  assert.ok(!record.url.includes('tok_partner_live'));
  assert.ok(record.url.includes('api_key=[REDACTED]'));
  assert.ok(record.url.includes('page=2'));
  assert.equal(result.redactedHeaders.Authorization, REDACTED);
  assert.ok(!JSON.stringify(relay.audit.records()).includes('tok_partner_live'));
  relay.close();
});

test('header and URL redaction handle any casing, userinfo, encoded values and path secrets', () => {
  const headers = redactHeaders({
    authorization: 'Bearer one',
    AUTHORIZATION: 'Bearer two',
    'X-Api-Key': 'key-three',
    Accept: 'application/json',
  });
  assert.equal(headers.authorization, REDACTED);
  assert.equal(headers.AUTHORIZATION, REDACTED);
  assert.equal(headers['X-Api-Key'], REDACTED);
  assert.equal(headers.Accept, 'application/json');

  const url = redactUrl(
    'https://user:pass@api.partner.com/tok%2Flive/v1?token=tok%2Flive&page=2',
    ['tok/live'],
  );
  assert.ok(!url.includes('user'));
  assert.ok(!url.includes('pass@'));
  assert.ok(!url.includes('tok%2Flive'));
  assert.ok(!url.includes('tok/live'));
  assert.ok(url.includes('page=2'));
});

test('surfaced transport errors do not contain credential values', async () => {
  const fake = createFakeTransport(() => {
    throw new Error('connection failed while sending tok_partner_live');
  });
  const relay = createRelay({
    bundle: makeBundle([PARTNER]),
    transport: fake.transport,
    maxAttempts: 1,
  });

  await assert.rejects(relay.request('https://api.partner.com/v1'), (error) => {
    assert.equal(error.code, 'TRANSPORT_ERROR');
    assert.ok(!error.message.includes('tok_partner_live'));
    assert.ok(error.message.includes(REDACTED));
    return true;
  });
  relay.close();
});

test('a queued retry re-derives credentials from the bundle active at send time', async () => {
  const fake = createFakeTransport([reply(503), reply(200)]);
  const relay = createRelay({
    bundle: makeBundle([PARTNER]),
    transport: fake.transport,
    baseDelayMs: 50,
    maxDelayMs: 50,
    random: () => 0.5,
  });

  const rotate = setTimeout(() => {
    relay.setBundle(
      compileBundle(
        { secrets: [{ ...PARTNER, id: 'partner-api-v2', value: 'tok_partner_rotated' }] },
        99,
      ),
    );
  }, 10);

  try {
    await relay.request('https://api.partner.com/v1');
  } finally {
    clearTimeout(rotate);
  }

  assert.equal(fake.callCount, 2);
  assert.equal(fake.sent[0].headers.Authorization, 'Bearer tok_partner_live');
  assert.equal(fake.sent[1].headers.Authorization, 'Bearer tok_partner_rotated');

  const attempts = relay.audit.records().filter((record) => record.type === 'attempt');
  assert.equal(attempts[0].id, 'partner-api');
  assert.equal(attempts[1].id, 'partner-api-v2');
  assert.equal(attempts[1].headers.Authorization, REDACTED);
  assert.ok(!JSON.stringify(relay.audit.records()).includes('tok_partner_live'));
  assert.ok(!JSON.stringify(relay.audit.records()).includes('tok_partner_rotated'));
  relay.close();
});

test('a retry whose entry disappears from the bundle sends no credential', async () => {
  const fake = createFakeTransport([reply(503), reply(200)]);
  const relay = createRelay({
    bundle: makeBundle([PARTNER]),
    transport: fake.transport,
    baseDelayMs: 50,
    maxDelayMs: 50,
    random: () => 0.5,
  });

  const remove = setTimeout(() => {
    relay.setBundle(compileBundle({ secrets: [] }, 100));
  }, 10);

  try {
    await relay.request('https://api.partner.com/v1');
  } finally {
    clearTimeout(remove);
  }

  assert.equal(fake.callCount, 2);
  assert.equal(fake.sent[1].headers.Authorization, undefined);
  relay.close();
});

test('temporal bounds are evaluated at send time with an injectable clock', async () => {
  const fake = createFakeTransport([reply(200)]);
  const relay = createRelay({
    bundle: makeBundle([
      {
        id: 'expired',
        host: 'expired.example.net',
        scheme: 'bearer',
        value: 'tok_expired',
        expires_at: '2020-01-01T00:00:00Z',
      },
      {
        id: 'future',
        host: 'future.example.net',
        scheme: 'bearer',
        value: 'tok_future',
        not_before: '2099-01-01T00:00:00Z',
      },
      {
        id: 'valid',
        host: 'valid.example.net',
        scheme: 'bearer',
        value: 'tok_valid',
        not_before: '2026-01-01T00:00:00Z',
        expires_at: '2026-12-31T23:59:59Z',
      },
    ]),
    transport: fake.transport,
    now: () => Date.parse('2026-06-01T00:00:00Z'),
  });

  await relay.request('https://expired.example.net/v1');
  await relay.request('https://future.example.net/v1');
  await relay.request('https://valid.example.net/v1');

  assert.equal(fake.sent[0].headers.Authorization, undefined);
  assert.equal(fake.sent[1].headers.Authorization, undefined);
  assert.equal(fake.sent[2].headers.Authorization, 'Bearer tok_valid');
  relay.close();
});

test('not_before is inclusive and expires_at is exclusive', async () => {
  const fake = createFakeTransport([reply(200)]);
  let nowMs = Date.parse('2026-01-01T00:00:00Z');
  const relay = createRelay({
    bundle: makeBundle([
      {
        id: 'bounded',
        host: 'bounded.example.net',
        scheme: 'bearer',
        value: 'tok_bounded',
        not_before: '2026-01-01T00:00:00Z',
        expires_at: '2026-02-01T00:00:00Z',
      },
    ]),
    transport: fake.transport,
    now: () => nowMs,
  });

  await relay.request('https://bounded.example.net/v1');
  assert.equal(fake.sent[0].headers.Authorization, 'Bearer tok_bounded');

  nowMs = Date.parse('2026-02-01T00:00:00Z');
  await relay.request('https://bounded.example.net/v1');
  assert.equal(fake.sent[1].headers.Authorization, undefined);

  relay.close();
});

test('a retry past expires_at does not replay the credential', async () => {
  const fake = createFakeTransport([reply(503), reply(200)]);
  let nowMs = Date.parse('2026-01-15T00:00:00Z');
  const relay = createRelay({
    bundle: makeBundle([
      {
        id: 'short-lived',
        host: 'api.partner.com',
        scheme: 'bearer',
        value: 'tok_short_lived',
        expires_at: '2026-01-16T00:00:00Z',
      },
    ]),
    transport: fake.transport,
    baseDelayMs: 50,
    maxDelayMs: 50,
    random: () => 0.5,
    now: () => nowMs,
  });

  const expire = setTimeout(() => {
    nowMs = Date.parse('2026-01-16T00:00:00Z');
  }, 10);

  try {
    await relay.request('https://api.partner.com/v1');
  } finally {
    clearTimeout(expire);
  }

  assert.equal(fake.callCount, 2);
  assert.equal(fake.sent[0].headers.Authorization, 'Bearer tok_short_lived');
  assert.equal(fake.sent[1].headers.Authorization, undefined);
  relay.close();
});
