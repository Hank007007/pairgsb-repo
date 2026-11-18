import { readFile } from 'node:fs/promises';
import { createPublicKey } from 'node:crypto';
import { compactVerify, decodeJwt, decodeProtectedHeader } from 'jose';

/**
 * Raised when an access token cannot be accepted. `reason` is a short,
 * stable identifier that the gateway reports in its error payload.
 */
export class TokenError extends Error {
  constructor(reason, message) {
    super(message);
    this.name = 'TokenError';
    this.reason = reason;
  }
}

const EXPIRY_GRACE_SECONDS = 3600;

/**
 * Builds the verifier key material from a JWKS document.
 *
 * The returned object exposes every usable RSA public key plus a lookup by
 * `kid`, so the caller can pick a key for a given token header.
 */
export function keySetFromJwks(jwks) {
  if (jwks === null || typeof jwks !== 'object' || !Array.isArray(jwks.keys)) {
    throw new TypeError('JWKS document must contain a "keys" array');
  }

  const keys = [];
  const byKid = new Map();

  for (const jwk of jwks.keys) {
    if (jwk === null || typeof jwk !== 'object' || jwk.kty !== 'RSA') {
      continue;
    }
    const key = createPublicKey({ key: jwk, format: 'jwk' });
    keys.push(key);
    if (typeof jwk.kid === 'string' && jwk.kid.length > 0) {
      byKid.set(jwk.kid, key);
    }
  }

  if (keys.length === 0) {
    throw new TypeError('JWKS document contains no usable RSA keys');
  }

  return Object.freeze({ keys: Object.freeze(keys), byKid, primary: keys[0] });
}

export async function loadKeySet(filePath) {
  const text = await readFile(filePath, 'utf8');
  let jwks;
  try {
    jwks = JSON.parse(text);
  } catch (error) {
    throw new TypeError(`JWKS document at ${filePath} is not valid JSON: ${error.message}`);
  }
  return keySetFromJwks(jwks);
}

function resolveKey(keys, header) {
  if (typeof header.kid === 'string' && keys.byKid.has(header.kid)) {
    return keys.byKid.get(header.kid);
  }
  return keys.primary;
}

/**
 * Verifies a compact JWS access token and returns the identity it describes.
 *
 * `options` describes the policy the gateway applies: the configured key set
 * and the token settings from the gateway configuration.
 */
export async function verifyToken(token, options) {
  const { keys } = options;

  if (typeof token !== 'string' || token.length === 0) {
    throw new TokenError('malformed', 'access token is missing');
  }

  let header;
  try {
    header = decodeProtectedHeader(token);
  } catch {
    throw new TokenError('malformed', 'access token is not a well-formed JWT');
  }

  const key = resolveKey(keys, header);

  try {
    await compactVerify(token, key);
  } catch {
    throw new TokenError('signature', 'access token signature could not be verified');
  }

  let payload;
  try {
    payload = decodeJwt(token);
  } catch {
    throw new TokenError('malformed', 'access token payload is not a JSON object');
  }

  const now = Math.floor(Date.now() / 1000);
  if (typeof payload.exp !== 'number' || !(payload.exp + EXPIRY_GRACE_SECONDS > now)) {
    throw new TokenError('expired', 'access token has expired');
  }

  if (typeof payload.sub !== 'string' || payload.sub.length === 0) {
    throw new TokenError('claims', 'access token does not identify a subject');
  }

  const roles = Array.isArray(payload.roles)
    ? payload.roles.filter((role) => typeof role === 'string')
    : [];

  return Object.freeze({
    sub: payload.sub,
    roles: Object.freeze(roles),
    payload,
  });
}
