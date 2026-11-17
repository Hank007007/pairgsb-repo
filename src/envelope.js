'use strict';

const { createHmac, timingSafeEqual } = require('node:crypto');

const codec = require('./codec');
const { resolveKey } = require('./key');
const { MAX_BYTES } = require('./limits');
const { EnvelopeFormatError, EnvelopeSignatureError } = require('./errors');

const ALGORITHM = 'sha256';

/**
 * Split an envelope into its two base64url segments.
 *
 * @param {string} token
 * @returns {{ payload: string, mac: string }}
 */
function splitEnvelope(token) {
  if (typeof token !== 'string') {
    throw new EnvelopeFormatError('envelope must be a string');
  }
  const segments = token.split('.');
  if (segments.length !== 2 || segments[0].length === 0 || segments[1].length === 0) {
    throw new EnvelopeFormatError('envelope must contain two non-empty segments');
  }
  return { payload: segments[0], mac: segments[1] };
}

/**
 * Compute the MAC over payload bytes.
 *
 * @param {Buffer} payloadBytes
 * @param {Buffer} key
 * @returns {Buffer}
 */
function mac(payloadBytes, key) {
  return createHmac(ALGORITHM, key).update(payloadBytes).digest();
}

/**
 * Compare two byte strings without exiting early at the first difference.
 */
function constantTimeEqual(left, right) {
  const length = Math.max(left.length, right.length);
  const paddedLeft = Buffer.alloc(length);
  const paddedRight = Buffer.alloc(length);
  left.copy(paddedLeft);
  right.copy(paddedRight);

  let equal;
  try {
    equal = timingSafeEqual(paddedLeft, paddedRight);
  } catch {
    equal = false;
  }
  return equal && left.length === right.length;
}

function asPayloadBytes(payload) {
  if (typeof payload === 'string') {
    return Buffer.from(payload, 'utf8');
  }
  if (Buffer.isBuffer(payload)) {
    return payload;
  }
  if (ArrayBuffer.isView(payload)) {
    return Buffer.from(payload.buffer, payload.byteOffset, payload.byteLength);
  }
  if (payload instanceof ArrayBuffer) {
    return Buffer.from(payload);
  }
  return codec.encodePayload(payload);
}

function asSignatureBytes(signature) {
  if (typeof signature === 'string') {
    try {
      return codec.b64urlDecode(signature);
    } catch {
      throw new EnvelopeSignatureError('envelope signature is unusable');
    }
  }
  if (Buffer.isBuffer(signature)) {
    return signature;
  }
  if (ArrayBuffer.isView(signature)) {
    return Buffer.from(signature.buffer, signature.byteOffset, signature.byteLength);
  }
  if (signature instanceof ArrayBuffer) {
    return Buffer.from(signature);
  }
  throw new EnvelopeSignatureError('envelope signature is unusable');
}

/**
 * Check the MAC supplied with an envelope against the payload bytes.
 *
 * @param {Buffer|Uint8Array|ArrayBuffer|string|Record<string, unknown>} payload
 * @param {Buffer} signature
 * @param {Buffer} key
 */
function verify(payload, signature, key) {
  const expected = mac(asPayloadBytes(payload), key);
  if (constantTimeEqual(expected, asSignatureBytes(signature))) {
    return;
  }
  throw new EnvelopeSignatureError('envelope signature does not match');
}

/**
 * Seal a session object into a signed envelope.
 *
 * @param {Record<string, unknown>} session
 * @param {{ key?: string|Buffer }} [options]
 * @returns {string}
 */
function seal(session, options = {}) {
  const key = resolveKey(options.key);
  const payloadBytes = codec.encodePayload(session);
  const signature = mac(payloadBytes, key);
  return `${codec.b64urlEncode(payloadBytes)}.${codec.b64urlEncode(signature)}`;
}

/**
 * Open a signed envelope and return the session it carries.
 *
 * @param {string} token
 * @param {{ key?: string|Buffer }} [options]
 * @returns {Record<string, unknown>}
 */
function open(token, options = {}) {
  const key = resolveKey(options.key);
  if (typeof token !== 'string') {
    throw new EnvelopeFormatError('envelope must be a string');
  }
  if (Buffer.byteLength(token, 'utf8') > MAX_BYTES) {
    throw new EnvelopeFormatError(`envelope exceeds ${MAX_BYTES} bytes`);
  }

  const segments = splitEnvelope(token);
  const signature = codec.b64urlDecode(segments.mac);
  const payloadBytes = codec.b64urlDecode(segments.payload);
  verify(payloadBytes, signature, key);
  return codec.decodePayload(payloadBytes);
}

module.exports = {
  ALGORITHM,
  splitEnvelope,
  mac,
  verify,
  seal,
  open,
};
