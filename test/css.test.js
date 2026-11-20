import test from 'node:test';
import assert from 'node:assert/strict';

import { sanitizeStyle, sanitizeStyleSheet } from '../src/index.js';

test('keeps allowlisted declarations with ordinary values', () => {
  const output = sanitizeStyle('fill: red; stroke-width: 2; opacity: .5');

  assert.match(output, /fill:\s*red/);
  assert.match(output, /stroke-width:\s*2/);
  assert.match(output, /opacity:\s*\.5/);
});

test('drops properties outside the allowlist', () => {
  assert.doesNotMatch(sanitizeStyle('margin: 0; fill: red'), /margin/);
  assert.match(sanitizeStyle('margin: 0; fill: red'), /fill:\s*red/);
});

test('keeps a safe absolute url()', () => {
  assert.equal(
    sanitizeStyle("background-image: url('https://cdn.example.com/pattern.png')"),
    "background-image: url('https://cdn.example.com/pattern.png')",
  );
});

test('keeps a stylesheet rule whose declarations are allowed', () => {
  const output = sanitizeStyleSheet('.bg { fill: red; stroke: blue }');

  assert.match(output, /\.bg\s*\{/);
  assert.match(output, /fill:\s*red/);
  assert.match(output, /stroke:\s*blue/);
});

test('returns an empty string for empty input', () => {
  assert.equal(sanitizeStyle(''), '');
  assert.equal(sanitizeStyleSheet(''), '');
});
