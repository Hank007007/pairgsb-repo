import { randomBytes } from "node:crypto";

/**
 * A fixed clock used by the tests: 2027-01-15T08:00:00Z, in milliseconds.
 */
export const NOW = 1_800_000_000_000;

export const NOW_SECONDS = Math.floor(NOW / 1000);

/**
 * Generate one key ring entry. Secrets are fresh for every test run.
 *
 * @param {string} kid
 * @param {{notBefore?: number, notAfter?: number}} [window]
 */
export function makeKey(kid, window = {}) {
  return { kid, secret: randomBytes(32), ...window };
}

/**
 * Generate a key ring with a current key and a grace-period key.
 */
export function makeKeyRing() {
  return [
    makeKey("2027-01", { notBefore: NOW_SECONDS - 3600 }),
    makeKey("2026-12", {
      notBefore: NOW_SECONDS - 90 * 24 * 3600,
      notAfter: NOW_SECONDS + 7 * 24 * 3600,
    }),
    makeKey("2026-11", {
      notBefore: NOW_SECONDS - 180 * 24 * 3600,
      notAfter: NOW_SECONDS - 10 * 24 * 3600,
    }),
  ];
}

/**
 * A claims object that is valid at NOW.
 *
 * @param {Record<string, unknown>} [overrides]
 */
export function makeClaims(overrides = {}) {
  return {
    iss: "login",
    sub: "user-42",
    aud: ["profile", "billing"],
    scopes: ["profile:read"],
    iat: NOW_SECONDS - 30,
    nbf: NOW_SECONDS - 30,
    exp: NOW_SECONDS + 900,
    jti: "7f2c1a90",
    ...overrides,
  };
}
