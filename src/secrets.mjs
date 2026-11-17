export class SecretError extends Error {
  constructor(message) {
    super(message);
    this.name = 'SecretError';
  }
}

const SECRET_PREFIX = 'RUNWRIGHT_SECRET_';

export class SecretStore {
  constructor(entries = {}) {
    this.values = new Map();
    for (const [name, value] of Object.entries(entries)) {
      if (value === undefined || value === null) continue;
      const text = String(value);
      if (text === '') continue;
      this.values.set(String(name).toUpperCase(), text);
    }
  }

  static fromEnvironment(environment = process.env, overrides = {}) {
    const entries = {};
    for (const [key, value] of Object.entries(environment)) {
      if (!key.startsWith(SECRET_PREFIX)) continue;
      const name = key.slice(SECRET_PREFIX.length);
      if (name) entries[name] = value;
    }
    for (const [name, value] of Object.entries(overrides)) {
      entries[name] = value;
    }
    return new SecretStore(entries);
  }

  has(name) {
    return this.values.has(String(name).toUpperCase());
  }

  get(name) {
    const key = String(name).toUpperCase();
    if (!this.values.has(key)) {
      throw new SecretError(`unknown secret "${key}"`);
    }
    return this.values.get(key);
  }

  names() {
    return [...this.values.keys()].sort();
  }

  entries() {
    return [...this.values.entries()];
  }

  mask(text) {
    let output = String(text);
    const candidates = [...this.values.values()].sort((a, b) => b.length - a.length);
    for (const secret of candidates) {
      output = output.split(secret).join('***');
    }
    return output;
  }
}
