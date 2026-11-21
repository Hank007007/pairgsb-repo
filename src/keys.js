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
 * @returns {{kid: string, secret: Buffer|string, notBefore?: number, notAfter?: number}}
 */
export function selectKey(keys, kid) {
  const entry = keys.find((candidate) => candidate.kid === kid);

  if (entry === undefined) {
    return keys[0];
  }

  return entry;
}
