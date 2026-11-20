'use strict';

const PREFIX = 'tm1';

function toBase64Url(text) {
  return Buffer.from(text, 'utf8').toString('base64url');
}

function fromBase64Url(encoded) {
  return Buffer.from(encoded, 'base64url').toString('utf8');
}

/**
 * Serialize a grant payload into the wire token format `tm1.<base64url(json)>`.
 *
 * @param {object} grant
 * @returns {string}
 */
function encodeGrant(grant) {
  if (grant === null || typeof grant !== 'object' || Array.isArray(grant)) {
    throw new TypeError('grant must be a plain object');
  }
  return `${PREFIX}.${toBase64Url(JSON.stringify(grant))}`;
}

/**
 * Parse a wire token back into its grant payload.
 *
 * @param {string} token
 * @returns {object}
 */
function decodeGrant(token) {
  if (typeof token !== 'string') {
    throw new TypeError('token must be a string');
  }
  const separator = token.indexOf('.');
  if (separator === -1) {
    throw new Error('malformed token: missing separator');
  }
  const prefix = token.slice(0, separator);
  if (prefix !== PREFIX) {
    throw new Error(`unsupported token prefix: ${prefix}`);
  }
  const encoded = token.slice(separator + 1);
  if (encoded.length === 0) {
    throw new Error('malformed token: empty payload');
  }

  let grant;
  try {
    grant = JSON.parse(fromBase64Url(encoded));
  } catch {
    throw new Error('malformed token: payload is not valid JSON');
  }
  if (grant === null || typeof grant !== 'object' || Array.isArray(grant)) {
    throw new Error('malformed token: payload must be an object');
  }
  return grant;
}

module.exports = { PREFIX, encodeGrant, decodeGrant };
