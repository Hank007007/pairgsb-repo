import crypto from 'node:crypto';

const FINGERPRINT_LENGTH = 12;

export function fingerprint(token) {
  if (typeof token !== 'string' || token.length === 0) {
    return null;
  }
  return crypto
    .createHash('sha256')
    .update(token, 'utf8')
    .digest('hex')
    .slice(0, FINGERPRINT_LENGTH);
}

export class AuditLog {
  constructor({ clock = () => new Date(), sink = null } = {}) {
    this.entries = [];
    this.clock = clock;
    this.sink = sink;
  }

  record(event = {}) {
    const entry = {
      at: this.clock().toISOString(),
      event: event.event ?? 'unknown',
      id: event.id ?? null,
      subject: event.subject ?? null,
      purpose: event.purpose ?? null,
      reason: event.reason ?? null,
      fingerprint: fingerprint(event.token),
    };

    this.entries.push(entry);

    if (typeof this.sink === 'function') {
      this.sink(entry);
    }

    return entry;
  }

  byEvent(name) {
    return this.entries.filter((entry) => entry.event === name);
  }

  toJSON() {
    return this.entries.map((entry) => ({ ...entry }));
  }
}
