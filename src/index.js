import { deflateRawSync } from "node:zlib";

import { assemble, encodeSegment, parseHeader, splitSegments } from "./envelope.js";
import { sign, verifyMac } from "./mac.js";
import { inflateBody, parseClaims } from "./decompress.js";
import {
  applyDefaults,
  assertAudience,
  checkLifetime,
  validateClaimTypes,
} from "./claims.js";
import { selectKey } from "./keys.js";
import { reject } from "./errors.js";

const DEFAULT_COMPRESSION = "deflate-raw";

/**
 * Seal a claims object into a v2 cookie.
 *
 * @param {Record<string, unknown>} claims
 * @param {{
 *   keys: Array<{kid: string, secret: Buffer|string, notBefore?: number, notAfter?: number}>,
 *   kid?: string,
 * }} options
 * @returns {string}
 */
export function issue(claims, options = {}) {
  const keys = options.keys;
  if (!Array.isArray(keys) || keys.length === 0) {
    throw reject("E_CONFIG", "a non-empty key ring is required");
  }
  if (claims === null || typeof claims !== "object" || Array.isArray(claims)) {
    throw reject("E_CLAIMS", "claims must be a plain object");
  }

  const kid = options.kid ?? keys[0].kid;
  const key = selectKey(keys, kid);

  const headerSegment = encodeSegment(
    Buffer.from(
      JSON.stringify({ kid, comp: DEFAULT_COMPRESSION }),
      "utf8",
    ),
  );
  const bodySegment = encodeSegment(
    deflateRawSync(Buffer.from(JSON.stringify(claims), "utf8")),
  );
  const macSegment = encodeSegment(sign(key.secret, bodySegment));

  return assemble(headerSegment, bodySegment, macSegment);
}

/**
 * Open a v2 cookie and return its claims.
 *
 * @param {string} cookie
 * @param {{
 *   keys: Array<{kid: string, secret: Buffer|string, notBefore?: number, notAfter?: number}>,
 *   service?: string,
 *   now?: number,
 * }} options
 * @returns {Record<string, unknown>}
 */
export function read(cookie, options = {}) {
  const keys = options.keys;
  if (!Array.isArray(keys) || keys.length === 0) {
    throw reject("E_CONFIG", "a non-empty key ring is required");
  }

  const segments = splitSegments(cookie);
  const header = parseHeader(segments.headerBytes);
  const key = selectKey(keys, header.kid);

  if (!verifyMac(key.secret, segments.bodySegment, segments.macBytes)) {
    throw reject("E_MAC", "cookie signature does not match");
  }

  const expanded = inflateBody(segments.bodyBytes);
  const claims = parseClaims(expanded);

  validateClaimTypes(claims);
  assertAudience(claims, options.service);
  checkLifetime(claims, options.now ?? Date.now());

  return applyDefaults(claims);
}
