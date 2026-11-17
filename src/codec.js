'use strict';

const { TextDecoder } = require('node:util');

const { EnvelopeFormatError } = require('./errors');
const { MAX_DEPTH, MAX_KEYS } = require('./limits');

const RESERVED_MEMBERS = new Set(['__proto__', 'constructor', 'prototype']);
const BASE64URL_PATTERN = /^[A-Za-z0-9_-]+$/;
const utf8Decoder = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true });

function isContainer(value) {
  return value !== null && typeof value === 'object';
}

function payloadBytes(bytes) {
  if (Buffer.isBuffer(bytes)) {
    return bytes;
  }
  if (ArrayBuffer.isView(bytes)) {
    return Buffer.from(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  }
  if (bytes instanceof ArrayBuffer) {
    return Buffer.from(bytes);
  }
  throw new EnvelopeFormatError('payload must be bytes or text');
}

/**
 * Enforce the payload limits from docs/envelope.md §6.
 */
function validateLimits(root) {
  const stack = [{ value: root, depth: 1 }];
  let memberNames = 0;

  while (stack.length > 0) {
    const { value, depth } = stack.pop();
    if (!isContainer(value)) {
      continue;
    }
    if (depth > MAX_DEPTH) {
      throw new EnvelopeFormatError(`payload nesting exceeds depth ${MAX_DEPTH}`);
    }

    if (Array.isArray(value)) {
      for (const element of value) {
        if (isContainer(element)) {
          stack.push({ value: element, depth: depth + 1 });
        }
      }
      continue;
    }

    const names = Object.keys(value);
    memberNames += names.length;
    if (memberNames > MAX_KEYS) {
      throw new EnvelopeFormatError(`payload has more than ${MAX_KEYS} member names`);
    }
    for (const name of names) {
      const member = value[name];
      if (isContainer(member)) {
        stack.push({ value: member, depth: depth + 1 });
      }
    }
  }
}

/**
 * Rebuild a fresh JSON value. Reserved member names are dropped at every
 * nesting level, before they can influence any prototype.
 */
function reconstruct(value) {
  if (Array.isArray(value)) {
    const result = new Array(value.length);
    for (let index = 0; index < value.length; index += 1) {
      result[index] = reconstruct(value[index]);
    }
    return result;
  }

  if (isContainer(value)) {
    const result = {};
    for (const name of Object.keys(value)) {
      if (RESERVED_MEMBERS.has(name)) {
        continue;
      }
      Object.defineProperty(result, name, {
        value: reconstruct(value[name]),
        enumerable: true,
        writable: true,
        configurable: true,
      });
    }
    return result;
  }

  return value;
}

/**
 * Turn payload bytes into a session object.
 *
 * @param {Buffer|Uint8Array|ArrayBuffer|string} bytes
 * @returns {Record<string, unknown>}
 */
function parseObject(bytes) {
  let text;
  if (typeof bytes === 'string') {
    text = bytes;
  } else {
    try {
      text = utf8Decoder.decode(payloadBytes(bytes));
    } catch (cause) {
      throw new EnvelopeFormatError('payload is not valid UTF-8', { cause });
    }
  }

  let parsed;
  try {
    parsed = JSON.parse(text);
  } catch (cause) {
    throw new EnvelopeFormatError('payload is not valid JSON', { cause });
  }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new EnvelopeFormatError('payload must be a JSON object');
  }

  validateLimits(parsed);
  return reconstruct(parsed);
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
 * @param {Buffer|Uint8Array|ArrayBuffer|string} bytes
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
 * Decode a strictly valid unpadded base64url segment into bytes.
 *
 * @param {string} segment
 * @returns {Buffer}
 */
function b64urlDecode(segment) {
  if (typeof segment !== 'string') {
    throw new EnvelopeFormatError('base64url segment must be a string');
  }
  if (segment.length === 0 || !BASE64URL_PATTERN.test(segment)) {
    throw new EnvelopeFormatError('segment is not valid base64url');
  }
  if (segment.length % 4 === 1) {
    throw new EnvelopeFormatError('segment is not valid base64url');
  }

  const bytes = Buffer.from(segment, 'base64url');
  if (b64urlEncode(bytes) !== segment) {
    throw new EnvelopeFormatError('segment is not canonical base64url');
  }
  return bytes;
}

module.exports = {
  parseObject,
  encodePayload,
  decodePayload,
  b64urlEncode,
  b64urlDecode,
};
