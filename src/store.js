import fsp from 'node:fs/promises';
import path from 'node:path';

import { toMillis } from './time.js';

/**
 * Storage used by the issue/consume flows.
 *
 * Callers hand the store the token that was presented to them; the store files
 * each record under the key returned by `#keyFor`.
 *
 * `remove` returns true when a record was present and false otherwise. It is
 * the storage primitive; deciding whether a removal is the one that wins a
 * race is the caller's responsibility.
 */

function emptyState() {
  return { records: {}, consumed: [] };
}

function normalizeState(raw) {
  if (raw === null || typeof raw !== 'object') {
    return emptyState();
  }
  const records =
    raw.records !== null && typeof raw.records === 'object' ? raw.records : {};
  const consumed = Array.isArray(raw.consumed) ? raw.consumed : [];
  return { records, consumed };
}

export class MemoryStore {
  #records = new Map();
  #consumed = new Set();
  #ready = Promise.resolve();

  constructor({ pepper = null } = {}) {
    this.pepper = pepper;
  }

  #keyFor(token) {
    return token;
  }

  async put(token, record) {
    await this.#ready;
    this.#records.set(this.#keyFor(token), structuredClone(record));
    return true;
  }

  async findByToken(token) {
    await this.#ready;
    const record = this.#records.get(this.#keyFor(token));
    return record ? structuredClone(record) : null;
  }

  async remove(token) {
    await this.#ready;
    const key = this.#keyFor(token);
    if (!this.#records.has(key)) {
      return false;
    }
    this.#records.delete(key);
    this.#consumed.add(key);
    return true;
  }

  async wasConsumed(token) {
    await this.#ready;
    return this.#consumed.has(this.#keyFor(token));
  }

  async pruneExpired(now) {
    await this.#ready;
    const cutoff = toMillis(now);
    let removed = 0;
    for (const [key, record] of this.#records) {
      if (toMillis(record.expiresAt) <= cutoff) {
        this.#records.delete(key);
        removed += 1;
      }
    }
    return removed;
  }

  async size() {
    await this.#ready;
    return this.#records.size;
  }

  async consumedCount() {
    await this.#ready;
    return this.#consumed.size;
  }

  async snapshot() {
    await this.#ready;
    const records = {};
    for (const [key, record] of this.#records) {
      records[key] = structuredClone(record);
    }
    return { records, consumed: [...this.#consumed] };
  }
}

export class FileStore {
  #loading = null;
  #state = null;

  constructor({ file, pepper = null, fs = fsp } = {}) {
    if (typeof file !== 'string' || file.length === 0) {
      throw new TypeError('FileStore requires a file path');
    }
    this.file = path.resolve(file);
    this.pepper = pepper;
    this.fs = fs;
  }

  #keyFor(token) {
    return token;
  }

  #ensureLoaded() {
    if (this.#state !== null) {
      return Promise.resolve(this.#state);
    }
    if (this.#loading === null) {
      this.#loading = this.#read().then(
        (state) => {
          this.#state = state;
          return state;
        },
        (error) => {
          this.#loading = null;
          throw error;
        },
      );
    }
    return this.#loading;
  }

  async #read() {
    let raw;
    try {
      raw = await this.fs.readFile(this.file, 'utf8');
    } catch (error) {
      if (error && error.code === 'ENOENT') {
        return emptyState();
      }
      throw error;
    }
    if (typeof raw !== 'string' || raw.trim() === '') {
      return emptyState();
    }
    return normalizeState(JSON.parse(raw));
  }

  async #flush() {
    const directory = path.dirname(this.file);
    await this.fs.mkdir(directory, { recursive: true });
    const temporary = `${this.file}.${process.pid}.tmp`;
    const body = `${JSON.stringify(this.#state, null, 2)}\n`;
    await this.fs.writeFile(temporary, body, 'utf8');
    await this.fs.rename(temporary, this.file);
  }

  async put(token, record) {
    const state = await this.#ensureLoaded();
    state.records[this.#keyFor(token)] = structuredClone(record);
    await this.#flush();
    return true;
  }

  async findByToken(token) {
    const state = await this.#ensureLoaded();
    const record = state.records[this.#keyFor(token)];
    return record ? structuredClone(record) : null;
  }

  async remove(token) {
    const state = await this.#ensureLoaded();
    const key = this.#keyFor(token);
    if (!Object.prototype.hasOwnProperty.call(state.records, key)) {
      return false;
    }
    delete state.records[key];
    if (!state.consumed.includes(key)) {
      state.consumed.push(key);
    }
    await this.#flush();
    return true;
  }

  async wasConsumed(token) {
    const state = await this.#ensureLoaded();
    return state.consumed.includes(this.#keyFor(token));
  }

  async pruneExpired(now) {
    const state = await this.#ensureLoaded();
    const cutoff = toMillis(now);
    let removed = 0;
    for (const [key, record] of Object.entries(state.records)) {
      if (toMillis(record.expiresAt) <= cutoff) {
        delete state.records[key];
        removed += 1;
      }
    }
    if (removed > 0) {
      await this.#flush();
    }
    return removed;
  }

  async size() {
    const state = await this.#ensureLoaded();
    return Object.keys(state.records).length;
  }

  async consumedCount() {
    const state = await this.#ensureLoaded();
    return state.consumed.length;
  }

  async snapshot() {
    const state = await this.#ensureLoaded();
    return structuredClone(state);
  }
}

export function createStore(kind, options = {}) {
  if (kind === 'memory') {
    return new MemoryStore(options);
  }
  if (kind === 'file') {
    return new FileStore(options);
  }
  throw new TypeError(`unknown store kind: ${String(kind)}`);
}
