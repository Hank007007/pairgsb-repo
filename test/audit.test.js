import test from 'node:test';
import assert from 'node:assert/strict';

import {
  REDACTED,
  redactHeaders,
  redactUrl,
  redactRecord,
  createAuditLog,
} from '../src/audit.js';

test('redactHeaders replaces authorization values', () => {
  const out = redactHeaders({ authorization: 'Bearer tok_live_123', accept: 'application/json' });
  assert.equal(out.authorization, REDACTED);
  assert.equal(out.accept, 'application/json');
});

test('redactHeaders replaces x-api-key values', () => {
  const out = redactHeaders({ 'x-api-key': 'key_live_456', 'content-type': 'application/json' });
  assert.equal(out['x-api-key'], REDACTED);
  assert.equal(out['content-type'], 'application/json');
});

test('redactHeaders leaves unrelated headers intact', () => {
  const input = { accept: '*/*', 'user-agent': 'credmux-tests/1.0' };
  assert.deepEqual(redactHeaders(input), input);
});

test('redactHeaders tolerates an empty map', () => {
  assert.deepEqual(redactHeaders(undefined), {});
  assert.deepEqual(redactHeaders({}), {});
});

test('redactUrl drops embedded userinfo', () => {
  assert.equal(redactUrl('https://user:pass@api.partner.com/v1'), 'https://api.partner.com/v1');
});

test('redactUrl leaves an ordinary URL untouched', () => {
  const url = 'https://api.partner.com/v1/orders?page=2';
  assert.equal(redactUrl(url), url);
});

test('redactUrl returns non-URL text as-is', () => {
  assert.equal(redactUrl('not a url'), 'not a url');
});

test('redactRecord applies header and URL passes', () => {
  const out = redactRecord({
    type: 'attempt',
    url: 'https://api.partner.com/v1',
    headers: { authorization: 'Bearer tok_live_123' },
  });
  assert.equal(out.headers.authorization, REDACTED);
  assert.equal(out.url, 'https://api.partner.com/v1');
  assert.equal(out.type, 'attempt');
});

test('audit log assigns a monotonic sequence number', () => {
  const log = createAuditLog();
  const first = log.write({ type: 'attempt', url: 'https://api.partner.com/1' });
  const second = log.write({ type: 'attempt', url: 'https://api.partner.com/2' });
  assert.equal(first.seq, 1);
  assert.equal(second.seq, 2);
  assert.equal(log.size(), 2);
});

test('audit log hands redacted records to the sink', () => {
  const seen = [];
  const log = createAuditLog({ sink: (record) => seen.push(record) });
  log.write({ type: 'attempt', url: 'https://api.partner.com/', headers: { authorization: 'Bearer abc' } });
  assert.equal(seen.length, 1);
  assert.equal(seen[0].headers.authorization, REDACTED);
});

test('audit log evicts old records past capacity', () => {
  const log = createAuditLog({ capacity: 2 });
  for (let i = 0; i < 4; i += 1) {
    log.write({ type: 'attempt', url: `https://api.partner.com/${i}` });
  }
  assert.equal(log.size(), 2);
  assert.equal(log.records()[1].seq, 4);
});

test('a throwing sink does not escape into the request path', () => {
  const log = createAuditLog({
    sink: () => {
      throw new Error('sink exploded');
    },
  });
  assert.doesNotThrow(() => log.write({ type: 'attempt', url: 'https://api.partner.com/' }));
  assert.equal(log.size(), 1);
});
