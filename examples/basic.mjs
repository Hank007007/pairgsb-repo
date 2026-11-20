import { AuditLog, createService, createStore } from '../src/index.js';

const pepper = process.env.TOKELATCH_PEPPER ?? 'demo-pepper';
const store = createStore('memory', { pepper });
const audit = new AuditLog();

const service = createService({
  store,
  pepper,
  audit,
  clock: () => new Date('2026-01-15T12:00:00.000Z'),
});

const issued = await service.issue({
  subject: 'alice@example.com',
  purpose: 'password-reset',
  meta: { requestedFrom: 'web' },
  ttlMs: 10 * 60 * 1000,
});

console.log('issued:', {
  id: issued.id,
  purpose: issued.purpose,
  expiresAt: issued.expiresAt,
});

const payload = await service.consume({
  token: issued.token,
  subject: 'alice@example.com',
  purpose: 'password-reset',
});

console.log('consumed:', {
  id: payload.id,
  subject: payload.subject,
  purpose: payload.purpose,
  meta: payload.meta,
});

try {
  await service.consume({
    token: issued.token,
    subject: 'alice@example.com',
    purpose: 'password-reset',
  });
} catch (error) {
  console.log('second attempt:', error.code);
}

console.log('audit events:', audit.toJSON().map((entry) => entry.event));
