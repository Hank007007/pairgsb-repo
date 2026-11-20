import assert from 'node:assert/strict';
import { test } from 'node:test';

import { AuditLog } from '../src/audit.js';
import { consume } from '../src/consume.js';
import { ERROR_CODES } from '../src/errors.js';
import { issue } from '../src/issue.js';
import { MemoryStore, FileStore } from '../src/store.js';

const PEPPER = 'test-pepper-0123456789';

function fixedClock(iso) {
  const instant = new Date(iso);
  return () => new Date(instant.getTime());
}

function makeContext(overrides = {}) {
  return {
    store: new MemoryStore({ pepper: PEPPER }),
    pepper: PEPPER,
    clock: fixedClock('2026-01-15T12:00:00.000Z'),
    audit: new AuditLog({ clock: fixedClock('2026-01-15T12:00:00.000Z') }),
    ...overrides,
  };
}

test('issue returns the token once and records the payload', async () => {
  const ctx = makeContext();

  const issued = await issue(ctx, {
    subject: 'alice@example.com',
    purpose: 'password-reset',
    meta: { requestIp: '203.0.113.7' },
    ttlMs: 10 * 60 * 1000,
  });

  assert.equal(typeof issued.token, 'string');
  assert.ok(issued.token.length >= 32);
  assert.equal(issued.subject, 'alice@example.com');
  assert.equal(issued.purpose, 'password-reset');
  assert.equal(issued.issuedAt, '2026-01-15T12:00:00.000Z');
  assert.equal(issued.expiresAt, '2026-01-15T12:10:00.000Z');

  const record = await ctx.store.findByToken(issued.token);
  assert.ok(record, 'the issued token should resolve to a record');
  assert.equal(record.subject, 'alice@example.com');
  assert.equal(record.purpose, 'password-reset');
  assert.deepEqual(record.meta, { requestIp: '203.0.113.7' });
  assert.equal(record.id, issued.id);
});

test('issue hands out a fresh token on every call', async () => {
  const ctx = makeContext();

  const first = await issue(ctx, {
    subject: 'alice@example.com',
    purpose: 'invite',
  });
  const second = await issue(ctx, {
    subject: 'alice@example.com',
    purpose: 'invite',
  });

  assert.notEqual(first.token, second.token);
  assert.notEqual(first.id, second.id);
});

test('issue applies the default lifetime when none is requested', async () => {
  const ctx = makeContext();

  const issued = await issue(ctx, {
    subject: 'bob@example.com',
    purpose: 'invite',
  });

  assert.equal(issued.expiresAt, '2026-01-15T12:15:00.000Z');
});

test('issue rejects malformed input before touching storage', async () => {
  const ctx = makeContext();

  await assert.rejects(
    issue(ctx, { subject: '   ', purpose: 'invite' }),
    { code: ERROR_CODES.INVALID_SUBJECT },
  );
  await assert.rejects(
    issue(ctx, { subject: 'carol@example.com', purpose: 'not-a-purpose' }),
    { code: ERROR_CODES.INVALID_PURPOSE },
  );
  await assert.rejects(
    issue(ctx, { subject: 'carol@example.com', purpose: 'invite', ttlMs: 0 }),
    { code: ERROR_CODES.INVALID_TTL },
  );
  await assert.rejects(
    issue(ctx, {
      subject: 'carol@example.com',
      purpose: 'invite',
      ttlMs: 8 * 24 * 60 * 60 * 1000,
    }),
    { code: ERROR_CODES.INVALID_TTL },
  );
  await assert.rejects(
    issue(ctx, { subject: 'carol@example.com', purpose: 'invite', meta: [] }),
    { code: ERROR_CODES.INVALID_META },
  );

  assert.equal(await ctx.store.size(), 0);
});

test('consume returns the stored payload and the token cannot be used twice', async () => {
  const ctx = makeContext();

  const issued = await issue(ctx, {
    subject: 'alice@example.com',
    purpose: 'password-reset',
    meta: { locale: 'en-GB' },
  });

  const payload = await consume(ctx, {
    token: issued.token,
    subject: 'alice@example.com',
    purpose: 'password-reset',
  });

  assert.equal(payload.id, issued.id);
  assert.equal(payload.subject, 'alice@example.com');
  assert.equal(payload.purpose, 'password-reset');
  assert.deepEqual(payload.meta, { locale: 'en-GB' });
  assert.equal(payload.consumedAt, '2026-01-15T12:00:00.000Z');

  await assert.rejects(
    consume(ctx, {
      token: issued.token,
      subject: 'alice@example.com',
      purpose: 'password-reset',
    }),
    { code: ERROR_CODES.TOKEN_ALREADY_USED },
  );
});

test('consume reports an unrecognised token', async () => {
  const ctx = makeContext();

  await assert.rejects(
    consume(ctx, {
      token: 'not-a-real-token',
      subject: 'alice@example.com',
      purpose: 'invite',
    }),
    { code: ERROR_CODES.TOKEN_UNKNOWN },
  );
});

test('consume requires a token', async () => {
  const ctx = makeContext();

  await assert.rejects(consume(ctx, { token: '' }), {
    code: ERROR_CODES.TOKEN_MISSING,
  });
});

test('audit log records issue and consume events', async () => {
  const ctx = makeContext();

  const issued = await issue(ctx, {
    subject: 'dave@example.com',
    purpose: 'api-key-handoff',
  });
  await consume(ctx, {
    token: issued.token,
    subject: 'dave@example.com',
    purpose: 'api-key-handoff',
  });

  const issuedEvents = ctx.audit.byEvent('token.issued');
  const consumedEvents = ctx.audit.byEvent('token.consumed');

  assert.equal(issuedEvents.length, 1);
  assert.equal(consumedEvents.length, 1);
  assert.equal(issuedEvents[0].id, issued.id);
  assert.equal(issuedEvents[0].subject, 'dave@example.com');
  assert.equal(consumedEvents[0].id, issued.id);
  assert.equal(consumedEvents[0].purpose, 'api-key-handoff');
});

test('file store round-trips a record through a fresh instance', async (t) => {
  const fs = await import('node:fs/promises');
  const os = await import('node:os');
  const path = await import('node:path');

  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'tokelatch-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const file = path.join(directory, 'tokens.json');

  const writer = makeContext({
    store: new FileStore({ file, pepper: PEPPER }),
  });

  const issued = await issue(writer, {
    subject: 'erin@example.com',
    purpose: 'invite',
    meta: { team: 'core' },
  });

  assert.equal(await writer.store.size(), 1);

  const readerCtx = makeContext({
    store: new FileStore({ file, pepper: PEPPER }),
  });

  const found = await readerCtx.store.findByToken(issued.token);
  assert.ok(found);
  assert.equal(found.subject, 'erin@example.com');

  const payload = await consume(readerCtx, {
    token: issued.token,
    subject: 'erin@example.com',
    purpose: 'invite',
  });
  assert.deepEqual(payload.meta, { team: 'core' });

  const afterConsume = makeContext({
    store: new FileStore({ file, pepper: PEPPER }),
  });
  assert.equal(await afterConsume.store.findByToken(issued.token), null);
  assert.equal(await afterConsume.store.wasConsumed(issued.token), true);
});

test('memory and file stores agree on the normal path', async (t) => {
  const fs = await import('node:fs/promises');
  const os = await import('node:os');
  const path = await import('node:path');

  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'tokelatch-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));

  const memory = makeContext();
  const onDisk = makeContext({
    store: new FileStore({
      file: path.join(directory, 'tokens.json'),
      pepper: PEPPER,
    }),
  });

  for (const ctx of [memory, onDisk]) {
    const issued = await issue(ctx, {
      subject: 'frank@example.com',
      purpose: 'password-reset',
    });
    const payload = await consume(ctx, {
      token: issued.token,
      subject: 'frank@example.com',
      purpose: 'password-reset',
    });
    assert.equal(payload.subject, 'frank@example.com');
  }
});
