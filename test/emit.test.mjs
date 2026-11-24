import test from 'node:test';
import assert from 'node:assert/strict';

import { emit_headers, serialize } from '../src/http/emit.mjs';

test('a single default-src reaches the header', () => {
  const headers = emit_headers("default-src 'self'");
  assert.match(headers['Content-Security-Policy'], /^default-src 'self'(?:;|$)/);
});

test('serialize joins directives with a semicolon and a space', () => {
  assert.equal(
    serialize({ 'default-src': ["'self'"], 'img-src': ['https://img.example'] }),
    "default-src 'self'; img-src https://img.example",
  );
});
