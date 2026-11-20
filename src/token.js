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
  return crypto.createHmac('sha256', pepper).update(token, 'utf8').digest('hex');
}

export function matches(expected, actual) {
  const expectedBuffer = Buffer.from(String(expected ?? ''), 'utf8');
  const actualBuffer = Buffer.from(String(actual ?? ''), 'utf8');
  if (expectedBuffer.length === 0) {
    return actualBuffer.length === 0;
  }
  if (expectedBuffer.length !== actualBuffer.length) {
    crypto.timingSafeEqual(expectedBuffer, expectedBuffer);
    return false;
  }
  return crypto.timingSafeEqual(expectedBuffer, actualBuffer);
}
