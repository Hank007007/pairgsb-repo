import { reject } from "./errors.js";

export const VERSION = "v2";

/** The only body compression algorithm defined by envelope v2. */
export const BODY_COMPRESSION = "deflate-raw";

/** Maximum size of the cookie string as received, in bytes. */
export const MAX_COOKIE_BYTES = 4096;

/** Maximum size of the decoded header JSON, in bytes. */
export const MAX_HEADER_BYTES = 1024;

/** Maximum size of the decompressed body, in bytes. */
export const MAX_BODY_BYTES = 65536;

const BASE64URL_PATTERN = /^[A-Za-z0-9_-]+$/;
const HEADER_TEXT_DECODER = new TextDecoder("utf-8", {
  fatal: true,
  ignoreBOM: true,
});

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
 * Decode one base64url segment, rejecting every non-canonical spelling.
 *
 * @param {string} segment
 * @returns {Buffer}
 */
export function decodeSegment(segment) {
  if (
    typeof segment !== "string" ||
    segment.length === 0 ||
    segment.length % 4 === 1 ||
    !BASE64URL_PATTERN.test(segment)
  ) {
    throw reject("E_ENCODING", "segment is not canonical base64url");
  }

  const bytes = Buffer.from(segment, "base64url");
  if (bytes.length === 0 || bytes.toString("base64url") !== segment) {
    throw reject("E_ENCODING", "segment is not canonical base64url");
  }

  return bytes;
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
  if (/[^\x00-\x7F]/.test(cookie)) {
    throw reject("E_FORMAT", "cookie must contain only ASCII characters");
  }
  if (/\s/.test(cookie)) {
    throw reject("E_FORMAT", "cookie must not contain whitespace");
  }

  const segments = cookie.split(".");
  if (segments.length !== 4) {
    throw reject("E_FORMAT", "cookie envelope must have exactly four segments");
  }
  if (segments[0] !== VERSION) {
    throw reject("E_FORMAT", "unsupported cookie envelope version");
  }

  const headerSegment = segments[1];
  const bodySegment = segments[2];
  const macSegment = segments[3];

  return {
    headerSegment,
    bodySegment,
    macSegment,
    headerBytes: decodeSegment(headerSegment),
    bodyBytes: decodeSegment(bodySegment),
    macBytes: decodeSegment(macSegment),
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
    value = JSON.parse(HEADER_TEXT_DECODER.decode(headerBytes));
  } catch {
    throw reject("E_HEADER", "cookie header is not valid JSON");
  }

  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw reject("E_HEADER", "cookie header must be a JSON object");
  }
  if (typeof value.kid !== "string" || value.kid.length === 0) {
    throw reject("E_HEADER", "cookie header must carry a non-empty kid");
  }
  if (value.comp !== BODY_COMPRESSION) {
    throw reject("E_HEADER", "cookie header carries an unsupported compression");
  }

  return value;
}
