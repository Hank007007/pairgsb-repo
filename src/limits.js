'use strict';

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

module.exports = {
  DEFAULTS,
  resolveLimits,
};