'use strict';

const { EnvelopeConfigError } = require('./errors');

const KEY_ENV = 'ENVWRAP_SECRET';

/**
 * Resolve the HMAC key for a call. An explicit key wins; otherwise the key is
 * read from the environment.
 *
 * @param {string|Buffer|undefined} key
 * @returns {Buffer}
 */
function resolveKey(key) {
  const value = key === undefined ? process.env[KEY_ENV] : key;
  if (value === undefined || value === null || value === '') {
    throw new EnvelopeConfigError(
      `no envelope key available: pass a key or set ${KEY_ENV}`
    );
  }
  if (Buffer.isBuffer(value) || ArrayBuffer.isView(value)) {
    const buffer = Buffer.isBuffer(value)
      ? value
      : Buffer.from(value.buffer, value.byteOffset, value.byteLength);
    if (buffer.length === 0) {
      throw new EnvelopeConfigError('envelope key must not be empty');
    }
    return buffer;
  }
  if (typeof value !== 'string') {
    throw new EnvelopeConfigError('envelope key must be a string or Buffer');
  }
  if (value.length === 0) {
    throw new EnvelopeConfigError('envelope key must not be empty');
  }
  return Buffer.from(value, 'utf8');
}

module.exports = {
  KEY_ENV,
  resolveKey,
};
