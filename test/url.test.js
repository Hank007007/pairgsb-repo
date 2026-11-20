import test from 'node:test';
import assert from 'node:assert/strict';

import { isSafeUrl } from '../src/index.js';

test('accepts http and https references', () => {
  assert.equal(isSafeUrl('https://cdn.example.com/avatar.png'), true);
  assert.equal(isSafeUrl('http://cdn.example.com/avatar.png'), true);
});

test('accepts image data URLs with a base64 payload', () => {
  assert.equal(isSafeUrl('data:image/png;base64,iVBORw0KGgo='), true);
  assert.equal(isSafeUrl('data:image/jpeg;base64,/9j/4AAQ'), true);
  assert.equal(isSafeUrl('data:image/webp;base64,UklGRg=='), true);
  assert.equal(isSafeUrl('data:image/gif;base64,R0lGODlhAQABAAAAACw='), true);
});

test('accepts a same-document fragment', () => {
  assert.equal(isSafeUrl('#gradient-1'), true);
});
