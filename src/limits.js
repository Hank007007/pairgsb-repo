'use strict';

/**
 * Envelope size limits, as stated in docs/envelope.md §6.
 */
const MAX_BYTES = 8 * 1024;
const MAX_DEPTH = 12;
const MAX_KEYS = 256;

module.exports = {
  MAX_BYTES,
  MAX_DEPTH,
  MAX_KEYS,
};
