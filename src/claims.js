import { reject } from "./errors.js";

const CLOCK_SKEW_SECONDS = 60;

const FORBIDDEN_CLAIM_NAMES = new Set(["__proto__", "constructor", "prototype"]);

const SESSION_DEFAULTS = Object.freeze({
  iss: "login",
  scopes: Object.freeze([]),
});

/**
 * Reject forbidden member names anywhere in a decoded claims value.
 *
 * @param {unknown} value
 */
export function assertNoForbiddenClaimNames(value) {
  const seen = new Set();
  const pending = [value];

  while (pending.length > 0) {
    const current = pending.pop();
    if (current === null || typeof current !== "object" || seen.has(current)) {
      continue;
    }
    seen.add(current);

    for (const name of Object.keys(current)) {
      if (FORBIDDEN_CLAIM_NAMES.has(name)) {
        throw reject("E_CLAIMS", `forbidden claim name ${name}`);
      }
      pending.push(current[name]);
    }
  }
}

/**
 * Complete a claims object with the library defaults.
 *
 * @param {Record<string, unknown>} claims
 * @param {Record<string, unknown>} [defaults]
 * @returns {Record<string, unknown>}
 */
export function applyDefaults(claims, defaults = SESSION_DEFAULTS) {
  if (defaults === null || typeof defaults !== "object" || Array.isArray(defaults)) {
    throw reject("E_CLAIMS", "claim defaults must be an object");
  }
  assertNoForbiddenClaimNames(defaults);

  const result = {};
  for (const [name, value] of Object.entries(defaults)) {
    if (!Object.hasOwn(claims, name)) {
      result[name] = structuredClone(value);
    }
  }
  for (const [name, value] of Object.entries(claims)) {
    result[name] = value;
  }

  return result;
}

/**
 * Enforce the declared types of the recognised claims.
 *
 * @param {Record<string, unknown>} claims
 */
export function validateClaimTypes(claims) {
  for (const name of ["iss", "sub", "jti"]) {
    if (claims[name] !== undefined && typeof claims[name] !== "string") {
      throw reject("E_CLAIMS", `claim ${name} must be a string`);
    }
  }

  if (claims.aud !== undefined) {
    const aud = claims.aud;
    const valid =
      typeof aud === "string" ||
      (Array.isArray(aud) && aud.every((entry) => typeof entry === "string"));
    if (!valid) {
      throw reject("E_CLAIMS", "claim aud must be a string or an array of strings");
    }
  }

  if (claims.scopes !== undefined) {
    const scopes = claims.scopes;
    if (!Array.isArray(scopes) || !scopes.every((entry) => typeof entry === "string")) {
      throw reject("E_CLAIMS", "claim scopes must be an array of strings");
    }
  }

  for (const name of ["exp", "iat", "nbf"]) {
    if (claims[name] !== undefined && !Number.isInteger(claims[name])) {
      throw reject("E_CLAIMS", `claim ${name} must be an integer number of seconds`);
    }
  }
}

/**
 * Check that a claims object names the reading service in its audience.
 *
 * @param {Record<string, unknown>} claims
 * @param {string|undefined} service
 */
export function assertAudience(claims, service) {
  if (claims.aud === undefined) {
    return;
  }

  if (typeof service !== "string" || service.length === 0) {
    throw reject("E_AUDIENCE", "a service identifier is required to read this cookie");
  }

  const audience = typeof claims.aud === "string" ? [claims.aud] : claims.aud;
  if (!audience.includes(service)) {
    throw reject("E_AUDIENCE", `service ${service} is not in the cookie audience`);
  }
}

/**
 * Check the claims against their validity window.
 *
 * @param {Record<string, unknown>} claims
 * @param {number} nowMs current time in milliseconds since the epoch
 */
export function checkLifetime(claims, nowMs) {
  if (!Number.isInteger(claims.exp)) {
    throw reject("E_CLAIMS", "claim exp is required and must be an integer");
  }

  const now = Math.floor(nowMs / 1000);

  if (claims.iat !== undefined && claims.iat > now + CLOCK_SKEW_SECONDS) {
    throw reject("E_CLAIMS", "claim iat lies in the future");
  }

  if (claims.nbf !== undefined) {
    if (claims.nbf >= claims.exp) {
      throw reject("E_CLAIMS", "claim nbf must be before claim exp");
    }
    if (now < claims.nbf) {
      throw reject("E_EXPIRED", "cookie is not valid yet");
    }
  }

  if (claims.iat !== undefined && claims.iat >= claims.exp) {
    throw reject("E_CLAIMS", "claim iat must be before claim exp");
  }

  if (now >= claims.exp) {
    throw reject("E_EXPIRED", "cookie has expired");
  }
}
