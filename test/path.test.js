import test from 'node:test';
import assert from 'node:assert/strict';

import { normalize, isInside } from '../src/index.js';

test('normalize keeps an absolute path unchanged', () => {
  assert.equal(normalize('/team/design'), '/team/design');
});

test('normalize collapses repeated separators and trailing slash', () => {
  assert.equal(normalize('/team//design/'), '/team/design');
});

test('normalize treats an empty path as root', () => {
  assert.equal(normalize(''), '/');
});

test('a nested path is inside its ancestor', () => {
  assert.equal(isInside('/team', '/team/design'), true);
});

test('a path is inside itself', () => {
  assert.equal(isInside('/team', '/team'), true);
});

test('root contains every path', () => {
  assert.equal(isInside('/', '/finance/q3.xlsx'), true);
});
