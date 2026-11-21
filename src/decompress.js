import { inflateRawSync } from "node:zlib";
import { MAX_BODY_BYTES } from "./envelope.js";
import { assertNoForbiddenClaimNames } from "./claims.js";
import { reject } from "./errors.js";

const BODY_TEXT_DECODER = new TextDecoder("utf-8", {
  fatal: true,
  ignoreBOM: true,
});

/**
 * Expand the compressed body of a cookie.
 *
 * @param {Buffer} compressed
 * @returns {Buffer}
 */
export function inflateBody(compressed) {
  try {
    return inflateRawSync(compressed, { maxOutputLength: MAX_BODY_BYTES });
  } catch (error) {
    if (
      error !== null &&
      typeof error === "object" &&
      error.code === "ERR_BUFFER_TOO_LARGE"
    ) {
      throw reject("E_TOO_LARGE", "cookie body exceeds the maximum decompressed size");
    }
    throw reject("E_INFLATE", "cookie body is not a valid deflate stream");
  }
}

/**
 * Decode the expanded body into a claims object.
 *
 * @param {Buffer} bytes
 * @returns {Record<string, unknown>}
 */
export function parseClaims(bytes) {
  let text;
  try {
    text = BODY_TEXT_DECODER.decode(bytes);
  } catch {
    throw reject("E_CLAIMS", "cookie body is not valid UTF-8");
  }

  let value;
  try {
    value = JSON.parse(text);
  } catch {
    throw reject("E_CLAIMS", "cookie body is not valid JSON");
  }

  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw reject("E_CLAIMS", "cookie claims must be a JSON object");
  }

  assertNoForbiddenClaimNames(value);

  return value;
}
