'use strict';

const { MultipartError } = require('./errors');

/** Default ceilings applied to one request. */
const DEFAULTS = Object.freeze({
  maxTotalBytes: 8 * 1024 * 1024,
  maxParts: 64,
  maxHeaderBytes: 16 * 1024,
  maxPartBytes: 4 * 1024 * 1024,
});

function positiveInteger(value, fallback, label) {
  if (value === undefined || value === null) return fallback;
  if (!Number.isInteger(value) || value <= 0) {
    throw new TypeError(`${label} must be a positive integer`);
  }
  return value;
}

/**
 * Merge caller-supplied limits over the defaults.
 */
function resolveLimits(overrides) {
  const source = overrides || {};
  return Object.freeze({
    maxTotalBytes: positiveInteger(source.maxTotalBytes, DEFAULTS.maxTotalBytes, 'maxTotalBytes'),
    maxParts: positiveInteger(source.maxParts, DEFAULTS.maxParts, 'maxParts'),
    maxHeaderBytes: positiveInteger(source.maxHeaderBytes, DEFAULTS.maxHeaderBytes, 'maxHeaderBytes'),
    maxPartBytes: positiveInteger(source.maxPartBytes, DEFAULTS.maxPartBytes, 'maxPartBytes'),
  });
}

/**
 * Byte accounting for one part.
 *
 * The parser hands over its request state together with the size of the body
 * bytes just seen. `state.part` is the part those bytes belong to.
 */
function accountBytes(state, size) {
  const part = state.part;
  if (!part) return;

  part.bytes += size;

  if (part.bytes > state.limits.maxPartBytes) {
    throw new MultipartError(
      413,
      'PART_TOO_LARGE',
      `part "${part.name || ''}" exceeds ${state.limits.maxPartBytes} bytes`
    );
  }
}

module.exports = {
  DEFAULTS,
  resolveLimits,
  accountBytes,
};
