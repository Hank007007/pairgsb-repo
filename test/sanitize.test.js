import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

import { sanitize } from '../src/index.js';

const fixturesDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'fixtures');

test('keeps allowlisted elements and their attributes', () => {
  const input = [
    '<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">',
    '  <title>Avatar</title>',
    '  <defs><linearGradient id="bg"><stop offset="0" stop-color="#4a90d9"/></linearGradient></defs>',
    '  <rect width="64" height="64" fill="url(#bg)"/>',
    '  <circle cx="32" cy="32" r="10" fill="#ffffff"/>',
    '  <image href="https://cdn.example.com/badge.png" x="44" y="4" width="16" height="16"/>',
    '</svg>',
  ].join('\n');

  const output = sanitize(input);

  assert.match(output, /<svg /);
  assert.match(output, /<title>Avatar<\/title>/);
  assert.match(output, /<linearGradient id="bg">/);
  assert.match(output, /<stop offset="0" stop-color="#4a90d9"\/>/);
  assert.match(output, /<rect width="64" height="64" fill="url\(#bg\)"\/>/);
  assert.match(output, /<circle cx="32" cy="32" r="10" fill="#ffffff"\/>/);
  assert.match(output, /<image href="https:\/\/cdn\.example\.com\/badge\.png"/);
});

test('preserves image data URLs backed by base64 payloads', () => {
  const payload = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';
  const input = `<svg xmlns="http://www.w3.org/2000/svg"><image href="data:image/png;base64,${payload}" width="1" height="1"/></svg>`;

  const output = sanitize(input);

  assert.match(output, /<image href="data:image\/png;base64,/);
});

test('keeps nested allowlisted groups and shape geometry', () => {
  const input = [
    '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32">',
    '<g transform="translate(2 2)">',
    '<path d="M2 2 L20 2 L20 20 Z" fill="#123456" fill-opacity="0.8"/>',
    '<polyline points="2,22 10,26 18,22" fill="none" stroke="#123456"/>',
    '</g>',
    '</svg>',
  ].join('');

  const output = sanitize(input);

  assert.match(output, /<g transform="translate\(2 2\)">/);
  assert.match(output, /<path d="M2 2 L20 2 L20 20 Z" fill="#123456" fill-opacity="0.8"\/>/);
  assert.match(output, /<polyline points="2,22 10,26 18,22" fill="none" stroke="#123456"\/>/);
});

test('escapes markup-significant characters in text content', () => {
  const input = '<svg xmlns="http://www.w3.org/2000/svg"><title>Tom & Jerry</title><desc>1 &lt; 2</desc></svg>';

  const output = sanitize(input);

  assert.match(output, /<title>Tom &amp; Jerry<\/title>/);
  assert.match(output, /<desc>1 &lt; 2<\/desc>/);
});

test('keeps the safe avatar fixture byte for byte', async () => {
  const input = await readFile(path.join(fixturesDir, 'safe-avatar.svg'), 'utf8');

  assert.equal(sanitize(input), input);
});

test('rejects input that is not a string', () => {
  assert.throws(() => sanitize(null), TypeError);
});
