import test from 'node:test';
import assert from 'node:assert/strict';

import { hmac } from '../src/sign.js';

/**
 * Cross-language anchor from docs/presign-format.md §2.1.
 *
 * The canonical string is written out here by hand (rather than produced by
 * the builder) so this vector is independent of the rest of the module: any
 * implementation that emits the documented seven lines and HMACs them with
 * SHA-256 under the same secret must produce this signature.
 */
const CANONICAL = [
  'urlseal-v1',
  'GET',
  'assets',
  'reports/2024/q1.pdf',
  'a=1&b=2',
  '1767225600',
  'demo',
].join('\n');

const EXPECTED_SIG = '2b4fe3f51c7b4f476ff79339ace9ba25f1e22c061a44f4bb7da236f64b9dacff';

test('canonical string follows the documented line layout', () => {
  assert.equal(CANONICAL.split('\n').length, 7);
  assert.ok(CANONICAL.startsWith('urlseal-v1\nGET\nassets\n'));
  assert.ok(CANONICAL.endsWith('\n1767225600\ndemo'));
});

test('hmac matches the documented cross-language vector', () => {
  assert.equal(hmac('demo-secret', CANONICAL), EXPECTED_SIG);
});
