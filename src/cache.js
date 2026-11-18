/**
 * A small least-recently-inserted cache with per-entry expiry.
 */
export class TtlCache {
  #entries = new Map();
  #ttlMs;
  #clock;
  #maxEntries;

  constructor({ ttlMs = 60_000, clock = Date.now, maxEntries = 500 } = {}) {
    if (!Number.isFinite(ttlMs) || ttlMs <= 0) {
      throw new RangeError('ttlMs must be a positive number');
    }
    if (!Number.isInteger(maxEntries) || maxEntries <= 0) {
      throw new RangeError('maxEntries must be a positive integer');
    }
    this.#ttlMs = ttlMs;
    this.#clock = clock;
    this.#maxEntries = maxEntries;
  }

  get ttlMs() {
    return this.#ttlMs;
  }

  get size() {
    return this.#entries.size;
  }

  has(key) {
    return this.get(key) !== undefined;
  }

  get(key) {
    const entry = this.#entries.get(key);
    if (entry === undefined) {
      return undefined;
    }
    if (entry.expiresAt <= this.#clock()) {
      this.#entries.delete(key);
      return undefined;
    }
    return entry.value;
  }

  set(key, value, ttlMs = this.#ttlMs) {
    if (!Number.isFinite(ttlMs) || ttlMs <= 0) {
      throw new RangeError('ttlMs must be a positive number');
    }
    this.#entries.delete(key);
    this.#entries.set(key, { value, expiresAt: this.#clock() + ttlMs });
    while (this.#entries.size > this.#maxEntries) {
      const oldest = this.#entries.keys().next().value;
      this.#entries.delete(oldest);
    }
    return this;
  }

  delete(key) {
    return this.#entries.delete(key);
  }

  clear() {
    this.#entries.clear();
  }
}
