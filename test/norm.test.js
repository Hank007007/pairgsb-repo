'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { PathError, isWithin, normalize } = require('../src/pathnorm/norm');

test('normalize: converts backslashes to forward slashes', () => {
  assert.equal(normalize('docs\\reports\\q3.txt'), 'docs/reports/q3.txt');
});

test('normalize: leaves an already-clean relative path unchanged', () => {
  assert.equal(normalize('docs/reports/q3.txt'), 'docs/reports/q3.txt');
});

test('normalize: leaves an already-clean absolute path unchanged', () => {
  assert.equal(normalize('/tenant-a/docs/q3.txt'), '/tenant-a/docs/q3.txt');
});

test('normalize: rejects a non-string input', () => {
  assert.throws(() => normalize(null), TypeError);
  assert.throws(() => normalize(42), TypeError);
});

test('PathError: carries a recognizable name', () => {
  const error = new PathError('outside root');
  assert.equal(error.name, 'PathError');
  assert.equal(error.message, 'outside root');
  assert.ok(error instanceof Error);
});

test('isWithin: accepts a direct child of the root', () => {
  assert.equal(isWithin('/tenant-a', '/tenant-a/docs/q3.txt'), true);
});

test('isWithin: accepts the root itself', () => {
  assert.equal(isWithin('/tenant-a', '/tenant-a'), true);
});
