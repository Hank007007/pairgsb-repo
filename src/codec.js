'use strict';

const { EnvelopeFormatError } = require('./errors');

/**
 * Merge a parsed JSON node into a fresh object, recursing into nested objects.
 */
function mergeInto(target, source) {
  for (const key of Object.keys(source)) {
    const value = source[key];
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      Object.assign(target, { [key]: mergeInto({}, value) });
    } else {
      Object.assign(target, { [key]: value });
    }
  }
  return target;
}

/**
 * Turn payload bytes into a session object.
 *
 * @param {Buffer|Uint8Array|string} bytes
 * @returns {Record<string, unknown>}
 */
function parseObject(bytes) {
  const text = Buffer.isBuffer(bytes) ? bytes.toString('utf8') : String(bytes);
  const parsed = JSON.parse(text);
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new EnvelopeFormatError('payload must be a JSON object');
  }
  return mergeInto({}, parsed);
}

/**
 * Serialize a session object to payload bytes.
 *
 * @param {unknown} session
 * @returns {Buffer}
 */
function encodePayload(session) {
  if (session === null || typeof session !== 'object' || Array.isArray(session)) {
    throw new EnvelopeFormatError('session must be a JSON object');
  }
  let text;
  try {
    text = JSON.stringify(session);
  } catch (cause) {
    throw new EnvelopeFormatError('session cannot be serialized as JSON', { cause });
  }
  if (typeof text !== 'string') {
    throw new EnvelopeFormatError('session cannot be serialized as JSON');
  }
  return Buffer.from(text, 'utf8');
}

/**
 * Decode payload bytes into a session object.
 *
 * @param {Buffer|Uint8Array|string} bytes
 * @returns {Record<string, unknown>}
 */
function decodePayload(bytes) {
  return parseObject(bytes);
}

/**
 * Encode bytes as unpadded base64url.
 *
 * @param {Buffer|Uint8Array|string} bytes
 * @returns {string}
 */
function b64urlEncode(bytes) {
  return Buffer.from(bytes).toString('base64url');
}

/**
 * Decode a base64url segment into bytes.
 *
 * @param {string} segment
 * @returns {Buffer}
 */
function b64urlDecode(segment) {
  const normalized = String(segment).replace(/-/g, '+').replace(/_/g, '/');
  return Buffer.from(normalized, 'base64');
}

module.exports = {
  parseObject,
  encodePayload,
  decodePayload,
  b64urlEncode,
  b64urlDecode,
};
