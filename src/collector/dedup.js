'use strict';

const DEFAULT_RETENTION_MS = 24 * 60 * 60 * 1000;

/**
 * Tracks which records a collector has already stored so that replays of the
 * same delivery do not land twice.
 */
class DedupTable {
  constructor(options = {}) {
    this.retentionMs = options.retentionMs ?? DEFAULT_RETENTION_MS;
    this.now = options.now || (() => Date.now());
    this.seen = new Map();
  }

  prune(now = this.now()) {
    const cutoff = now - this.retentionMs;
    for (const [key, at] of this.seen) {
      if (at < cutoff) this.seen.delete(key);
    }
  }

  has(entry) {
    this.prune();
    return this.seen.has(String(entry.seq));
  }

  /**
   * Records `entry`. Returns true when the entry had not been seen before,
   * false when it was already present.
   */
  accept(entry) {
    const now = entry.at ?? this.now();
    this.prune(now);
    const key = String(entry.seq);
    if (this.seen.has(key)) {
      this.seen.set(key, now);
      return false;
    }
    this.seen.set(key, now);
    return true;
  }

  get size() {
    return this.seen.size;
  }
}

module.exports = {
  DEFAULT_RETENTION_MS,
  DedupTable,
};
