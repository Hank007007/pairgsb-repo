import { createHmac, timingSafeEqual } from 'node:crypto';

const DIGEST_HEX_LENGTH = 64;

export function hashSecret(pepper, secret) {
  return createHmac('sha256', pepper).update(secret, 'utf8').digest('hex');
}

export function digestsMatch(actualHex, expectedHex) {
  const actual = Buffer.from(actualHex, 'hex');
  const wellFormed =
    typeof expectedHex === 'string' &&
    expectedHex.length === DIGEST_HEX_LENGTH &&
    /^[0-9a-f]+$/.test(expectedHex);
  const expected = wellFormed ? Buffer.from(expectedHex, 'hex') : Buffer.alloc(actual.length);
  const equal = timingSafeEqual(actual, expected);
  return wellFormed && equal;
}
