/**
 * Request authentication.
 *
 * A request is signed over the body its sender put on the wire. Verification
 * sits between decoding and dispatch: the body must decode with the resolved
 * schema, and the signature must match.
 */
import { createHmac, timingSafeEqual } from 'node:crypto';
import { ProtolensError, CODES } from '../errors.js';
import { decodeMessage } from '../wire/decode.js';
import { encodeMessage } from '../wire/encode.js';

/**
 * @typedef {object} SignedRequest
 * @property {Buffer|Uint8Array} body raw request body
 * @property {Buffer|Uint8Array|string} signature HMAC of the body
 * @property {string} type message type name
 * @property {number} [version] schema version the sender encoded against
 * @property {Record<string,string>} [headers] transport headers
 */

/**
 * Compute the HMAC of `data` for a key.
 *
 * @param {string|Buffer} secret
 * @param {Buffer|Uint8Array|string} data
 * @returns {Buffer}
 */
export function signRequest(secret, data) {
  return createHmac('sha256', secret).update(data).digest();
}

function signatureToBuffer(signature) {
  if (Buffer.isBuffer(signature)) return signature;
  if (signature instanceof Uint8Array) return Buffer.from(signature);
  if (typeof signature === 'string') return Buffer.from(signature, 'hex');
  return Buffer.alloc(0);
}

/**
 * Verify a signed request: decode it with its schema, then check its HMAC.
 *
 * @param {SignedRequest} request
 * @param {object} ctx `{ registry, secret }`
 * @returns {{ message: object, schema: object, version: number }}
 */
export function verifyRequest(request, ctx) {
  const { registry, secret } = ctx;
  const body = Buffer.isBuffer(request.body) ? request.body : Buffer.from(request.body);
  const schema = ctx.resolve(request.type, request.version);
  const message = decodeMessage(body, schema, { getType: schema.getType });

  const expected = signRequest(secret, encodeMessage(message, schema));
  const actual = signatureToBuffer(request.signature);

  if (!timingSafeEqual(actual, expected)) {
    throw new ProtolensError(CODES.SIGNATURE_INVALID, 'request signature does not match', {
      type: request.type,
      version: schema.version,
    });
  }

  return { message, schema, version: schema.version };
}
