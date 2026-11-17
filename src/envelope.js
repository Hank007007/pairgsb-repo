'use strict';

const { createHmac } = require('node:crypto');

const codec = require('./codec');
const { resolveKey } = require('./key');
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
 * Check the MAC supplied with an envelope against the session it carries.
 *
 * @param {Record<string, unknown>} session
 * @param {Buffer} signature
 * @param {Buffer} key
 */
function verify(session, signature, key) {
  const expected = mac(codec.encodePayload(session), key);
  if (signature.toString('base64url') === expected.toString('base64url')) {
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
  const segments = splitEnvelope(token);
  const payloadBytes = codec.b64urlDecode(segments.payload);
  const session = codec.decodePayload(payloadBytes);
  const signature = codec.b64urlDecode(segments.mac);
  verify(session, signature, key);
  return session;
}

module.exports = {
  ALGORITHM,
  splitEnvelope,
  mac,
  verify,
  seal,
  open,
};
