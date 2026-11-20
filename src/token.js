import crypto from 'node:crypto';

const TOKEN_BYTES = 32;

export function generateToken() {
  return crypto.randomBytes(TOKEN_BYTES).toString('base64url');
}

export function digest(token, pepper) {
  if (typeof token !== 'string' || token.length === 0) {
    throw new TypeError('token must be a non-empty string');
  }
  if (typeof pepper !== 'string' || pepper.length === 0) {
    throw new TypeError('pepper must be a non-empty string');
  }
  return crypto.createHash('sha256').update(token, 'utf8').digest('hex');
}

export function matches(expected, actual) {
  return expected === actual;
}
