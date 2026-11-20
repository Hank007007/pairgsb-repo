'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { splitFields, createTokenizer } = require('../src/tokenizer');

test('splits a simple comma separated line', () => {
  assert.deepEqual(splitFields('a,b,c', ','), ['a', 'b', 'c']);
});

test('splits on tabs for tab separated input', () => {
  assert.deepEqual(splitFields('a\tb\tc', '\t'), ['a', 'b', 'c']);
});

test('keeps a trailing empty field', () => {
  assert.deepEqual(splitFields('a,b,', ','), ['a', 'b', '']);
});

test('reads a quoted field containing the delimiter', () => {
  assert.deepEqual(splitFields('"Acme, Inc.",sales', ','), ['Acme, Inc.', 'sales']);
});

test('a quoted field may contain leading and trailing spaces', () => {
  assert.deepEqual(splitFields('"  padded  ",next', ','), ['  padded  ', 'next']);
});

test('a plain quote inside an unquoted field is data', () => {
  assert.deepEqual(splitFields('5" screen,monitor', ','), ['5" screen', 'monitor']);
});

test('tokenizer walks records until exhausted', () => {
  const tokenizer = createTokenizer('a,b\nc,d\n', ',');

  assert.deepEqual(tokenizer.nextRecord(), ['a', 'b']);
  assert.deepEqual(tokenizer.nextRecord(), ['c', 'd']);
  assert.equal(tokenizer.nextRecord(), null);
});

test('tokenizer handles a final record without a terminator', () => {
  const tokenizer = createTokenizer('a,b\nc,d', ',');

  assert.deepEqual(tokenizer.nextRecord(), ['a', 'b']);
  assert.deepEqual(tokenizer.nextRecord(), ['c', 'd']);
  assert.equal(tokenizer.nextRecord(), null);
});
