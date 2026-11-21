import { reject } from "./errors.js";

export const VERSION = "v2";

/** Maximum size of the cookie string as received, in bytes. */
export const MAX_COOKIE_BYTES = 4096;

/** Maximum size of the decoded header JSON, in bytes. */
export const MAX_HEADER_BYTES = 1024;

/**
 * Encode raw bytes as an unpadded base64url segment.
 *
 * @param {Buffer|Uint8Array} bytes
 * @returns {string}
 */
export function encodeSegment(bytes) {
  return Buffer.from(bytes).toString("base64url");
}

/**
 * Join the four segments of a v2 cookie.
 *
 * @param {string} headerSegment
 * @param {string} bodySegment
 * @param {string} macSegment
 * @returns {string}
 */
export function assemble(headerSegment, bodySegment, macSegment) {
  return [VERSION, headerSegment, bodySegment, macSegment].join(".");
}

/**
 * Split a cookie into its segments and decode each segment into bytes.
 *
 * @param {string} cookie
 * @returns {{
 *   headerSegment: string,
 *   bodySegment: string,
 *   macSegment: string,
 *   headerBytes: Buffer,
 *   bodyBytes: Buffer,
 *   macBytes: Buffer,
 * }}
 */
export function splitSegments(cookie) {
  if (typeof cookie !== "string" || cookie.length === 0) {
    throw reject("E_FORMAT", "cookie must be a non-empty string");
  }
  if (Buffer.byteLength(cookie, "utf8") > MAX_COOKIE_BYTES) {
    throw reject("E_TOO_LARGE", "cookie exceeds the maximum length");
  }

  const segments = cookie.split(".");
  if (segments[0] !== VERSION) {
    throw reject("E_FORMAT", "unsupported cookie envelope version");
  }
  if (segments.length < 4) {
    throw reject("E_FORMAT", "cookie envelope is missing segments");
  }

  const headerSegment = segments[1];
  const bodySegment = segments[2];
  const macSegment = segments[3];

  return {
    headerSegment,
    bodySegment,
    macSegment,
    headerBytes: Buffer.from(headerSegment, "base64"),
    bodyBytes: Buffer.from(bodySegment, "base64"),
    macBytes: Buffer.from(macSegment, "base64"),
  };
}

/**
 * Parse the decoded header bytes into a plain object.
 *
 * @param {Buffer} headerBytes
 * @returns {Record<string, unknown>}
 */
export function parseHeader(headerBytes) {
  if (headerBytes.length > MAX_HEADER_BYTES) {
    throw reject("E_TOO_LARGE", "cookie header exceeds the maximum length");
  }

  let value;
  try {
    value = JSON.parse(headerBytes.toString("utf8"));
  } catch {
    throw reject("E_HEADER", "cookie header is not valid JSON");
  }

  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw reject("E_HEADER", "cookie header must be a JSON object");
  }

  return value;
}
