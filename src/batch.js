'use strict';

const crypto = require('node:crypto');

/**
 * Assemble batches from queued records and compute their digest.
 */
class BatchBuilder {
  constructor(options = {}) {
    this.source = options.source === undefined ? 'node' : options.source;
    this.counter = 0;
    this.lastDigest = null;
  }

  nextBatchId() {
    this.counter += 1;
    return `${this.source}-${this.counter}`;
  }

  /**
   * Build one batch from `records`.
   */
  buildBatch(records) {
    const ordered = records.slice().sort((a, b) => a.ts - b.ts);

    const prevDigest = this.lastDigest;
    const digest = crypto
      .createHash('sha256')
      .update(ordered.map((record) => record.payload).join(''))
      .digest('hex');

    const batch = {
      batchId: this.nextBatchId(),
      source: this.source,
      prevDigest,
      records: ordered,
      digest,
    };

    this.lastDigest = digest;
    return batch;
  }
}

/**
 * Convenience wrapper for a one-off batch.
 */
function buildBatch(records, options = {}) {
  const builder = new BatchBuilder(options);
  return builder.buildBatch(records);
}

function sha256Hex(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

module.exports = {
  BatchBuilder,
  buildBatch,
  sha256Hex,
};
