import { createHmac } from "node:crypto";

const ALGORITHM = "sha256";

/**
 * Compute the authentication tag for a cookie.
 *
 * @param {Buffer|string} secret
 * @param {string} bodySegment
 * @returns {Buffer} 32-byte HMAC-SHA256 tag
 */
export function sign(secret, bodySegment) {
  return createHmac(ALGORITHM, secret).update(bodySegment, "ascii").digest();
}

/**
 * Check a received tag against the tag derived from the cookie body.
 *
 * @param {Buffer|string} secret
 * @param {string} bodySegment
 * @param {Buffer} tag
 * @returns {boolean}
 */
export function verifyMac(secret, bodySegment, tag) {
  const expected = createHmac(ALGORITHM, secret)
    .update(bodySegment, "ascii")
    .digest("hex");

  return expected === tag.toString("hex");
}
