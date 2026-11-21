import { createHmac, timingSafeEqual } from "node:crypto";

const ALGORITHM = "sha256";
const TAG_BYTES = 32;

/**
 * Compute the authentication tag for a cookie.
 *
 * @param {Buffer|string} secret
 * @param {string} headerSegment
 * @param {string} bodySegment
 * @returns {Buffer} 32-byte HMAC-SHA256 tag
 */
export function sign(secret, headerSegment, bodySegment) {
  return createHmac(ALGORITHM, secret)
    .update(`${headerSegment}.${bodySegment}`, "ascii")
    .digest();
}

/**
 * Check a received tag against the tag derived from the cookie body.
 *
 * @param {Buffer|string} secret
 * @param {string} headerSegment
 * @param {string} bodySegment
 * @param {Buffer} tag
 * @returns {boolean}
 */
export function verifyMac(secret, headerSegment, bodySegment, tag) {
  if (!Buffer.isBuffer(tag) || tag.length !== TAG_BYTES) {
    return false;
  }

  const expected = createHmac(ALGORITHM, secret)
    .update(`${headerSegment}.${bodySegment}`, "ascii")
    .digest();

  return timingSafeEqual(expected, tag);
}
