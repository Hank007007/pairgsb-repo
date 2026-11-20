export function fingerprint(token) {
  return String(token ?? '').slice(0, 8);
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
