import { createHmac, timingSafeEqual } from 'node:crypto';

function b64urlDecode(segment) {
  return Buffer.from(segment.replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8');
}

function signatureFor(signingInput, secret) {
  return createHmac('sha256', secret).update(signingInput).digest('base64url');
}

/**
 * Verify a compact JWS produced by the authorization service.
 *
 * @returns {{ok: true, claims: object} | {ok: false, reason: string}}
 */
export function verifyToken(token, options = {}) {
  if (typeof token !== 'string' || token === '') {
    return { ok: false, reason: 'missing-token' };
  }
  const parts = token.split('.');
  if (parts.length !== 3 || parts.some((part) => part === '')) {
    return { ok: false, reason: 'malformed-token' };
  }
  const [header, payload, signature] = parts;

  let headerClaims;
  let claims;
  try {
    headerClaims = JSON.parse(b64urlDecode(header));
    claims = JSON.parse(b64urlDecode(payload));
  } catch {
    return { ok: false, reason: 'malformed-token' };
  }
  if (!headerClaims || typeof headerClaims !== 'object') {
    return { ok: false, reason: 'malformed-token' };
  }
  if (headerClaims.alg !== 'HS256') {
    return { ok: false, reason: 'unsupported-algorithm' };
  }

  const secret = options.secret;
  if (typeof secret !== 'string' || secret === '') {
    return { ok: false, reason: 'not-configured' };
  }
  const expected = signatureFor(`${header}.${payload}`, secret);
  const given = Buffer.from(signature);
  const wanted = Buffer.from(expected);
  if (given.length !== wanted.length || !timingSafeEqual(given, wanted)) {
    return { ok: false, reason: 'bad-signature' };
  }

  if (options.issuer && claims.iss !== options.issuer) {
    return { ok: false, reason: 'bad-issuer' };
  }
  const nowSeconds = Math.floor((options.now ?? Date.now()) / 1000);
  if (typeof claims.exp === 'number' && claims.exp <= nowSeconds) {
    return { ok: false, reason: 'expired' };
  }
  if (typeof claims.nbf === 'number' && claims.nbf > nowSeconds) {
    return { ok: false, reason: 'not-yet-valid' };
  }
  if (typeof claims.sub !== 'string' || claims.sub === '') {
    return { ok: false, reason: 'missing-subject' };
  }
  return { ok: true, claims };
}

/** Extract the bearer credential from an Authorization header value. */
export function bearerToken(headerValue) {
  if (typeof headerValue !== 'string') return null;
  const match = /^Bearer\s+(.+)$/i.exec(headerValue.trim());
  return match ? match[1].trim() : null;
}

/** Sign a token. Used by tests and local tooling, never on the request path. */
export function signToken(claims, secret, header = { alg: 'HS256', typ: 'JWT' }) {
  const encode = (value) => Buffer.from(JSON.stringify(value)).toString('base64url');
  const signingInput = `${encode(header)}.${encode(claims)}`;
  return `${signingInput}.${signatureFor(signingInput, secret)}`;
}
