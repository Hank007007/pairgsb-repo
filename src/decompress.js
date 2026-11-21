import { inflateRawSync } from "node:zlib";
import { reject } from "./errors.js";

/**
 * Expand the compressed body of a cookie.
 *
 * @param {Buffer} compressed
 * @returns {Buffer}
 */
export function inflateBody(compressed) {
  try {
    return inflateRawSync(compressed);
  } catch {
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
    text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
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

  return value;
}
