import assert from 'node:assert/strict';
import test from 'node:test';

import { compileGlob, matchGlob } from '../src/glob.js';

test('compileGlob returns an anchored regular expression', () => {
  const compiled = compileGlob('/healthz');
  assert.ok(compiled instanceof RegExp);
  assert.equal(compiled.test('/healthz'), true);
});

test('a literal pattern matches only itself, case sensitively', () => {
  assert.equal(matchGlob('/healthz', '/healthz'), true);
  assert.equal(matchGlob('/healthz', '/health'), false);
  assert.equal(matchGlob('/docs', '/Docs'), false);
});

test('a wildcard segment matches a path segment', () => {
  assert.equal(matchGlob('/docs/*', '/docs/guide'), true);
  assert.equal(matchGlob('/users/*', '/users/alice'), true);
  assert.equal(matchGlob('/users/*', '/users/bob'), true);
});

test('regular expression metacharacters are treated as literals', () => {
  assert.equal(matchGlob('/v1.2/notes', '/v1.2/notes'), true);
  assert.equal(matchGlob('/v1.2/notes', '/v1x2/notes'), false);
  assert.equal(matchGlob('/a+b/notes', '/a+b/notes'), true);
});

test('a non-string pattern is rejected', () => {
  assert.throws(() => compileGlob(42), TypeError);
  assert.throws(() => compileGlob(''), TypeError);
});
