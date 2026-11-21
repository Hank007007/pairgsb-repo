import { reject } from "./errors.js";

/**
 * Resolve the key entry used for a cookie.
 *
 * The ring is ordered newest first. Entries are plain objects:
 *
 *   { kid: string, secret: Buffer|string, notBefore?: number, notAfter?: number }
 *
 * `notBefore` and `notAfter` are seconds since the Unix epoch.
 *
 * @param {Array<{kid: string, secret: Buffer|string, notBefore?: number, notAfter?: number}>} keys
 * @param {string|undefined} kid
 * @param {number|undefined} nowSeconds current time in seconds
 * @returns {{kid: string, secret: Buffer|string, notBefore?: number, notAfter?: number}}
 */
export function selectKey(keys, kid, nowSeconds) {
  if (typeof kid !== "string" || kid.length === 0) {
    throw reject("E_UNKNOWN_KID", "cookie header does not name a key");
  }

  const matches = keys.filter((candidate) => candidate.kid === kid);
  if (matches.length === 0) {
    throw reject("E_UNKNOWN_KID", "cookie names an unknown key");
  }
  if (matches.length > 1) {
    throw reject("E_CONFIG", "key ring contains a duplicate key id");
  }

  const entry = matches[0];
  if (entry.notBefore !== undefined && !Number.isFinite(entry.notBefore)) {
    throw reject("E_CONFIG", "key notBefore must be a finite number of seconds");
  }
  if (entry.notAfter !== undefined && !Number.isFinite(entry.notAfter)) {
    throw reject("E_CONFIG", "key notAfter must be a finite number of seconds");
  }
  if (
    entry.notBefore !== undefined &&
    entry.notAfter !== undefined &&
    entry.notBefore >= entry.notAfter
  ) {
    throw reject("E_CONFIG", "key validity window must not be empty");
  }

  if (nowSeconds !== undefined) {
    if (entry.notBefore !== undefined && nowSeconds < entry.notBefore) {
      throw reject("E_KEY_EXPIRED", "selected key is not valid yet");
    }
    if (entry.notAfter !== undefined && nowSeconds >= entry.notAfter) {
      throw reject("E_KEY_EXPIRED", "selected key is no longer valid");
    }
  }

  return entry;
}
