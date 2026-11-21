/**
 * Error type shared by the envelope, key, compression, and claims layers.
 *
 * Every rejected cookie is surfaced as a CookieError carrying a stable
 * machine-readable code. Callers should treat any thrown value as a
 * rejection and must not fall back to unverified data.
 */
export class CookieError extends Error {
  constructor(code, message) {
    super(message);
    this.name = "CookieError";
    this.code = code;
  }
}

export function reject(code, message) {
  return new CookieError(code, message);
}
