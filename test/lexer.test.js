'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { tokenize } = require('../src/lexer');
const { RuleError } = require('../src/errors');

function types(source) {
  return tokenize(source).map((token) => token.type);
}

function values(source) {
  return tokenize(source)
    .filter((token) => token.type !== 'eof')
    .map((token) => token.value);
}

test('tokenizes numbers, strings, identifiers and operators', () => {
  assert.deepEqual(
    values('severity == "high" and source.port >= 443'),
    ['severity', '==', 'high', 'and', 'source', '.', 'port', '>=', 443],
  );
});

test('distinguishes two-character operators from single-character ones', () => {
  assert.deepEqual(values('a == b != c <= d >= e < f > g'), [
    'a', '==', 'b', '!=', 'c', '<=', 'd', '>=', 'e', '<', 'f', '>', 'g',
  ]);
});

test('recognises integers and decimals', () => {
  const tokens = tokenize('0 12 3.5');
  assert.equal(tokens[0].value, 0);
  assert.equal(tokens[1].value, 12);
  assert.equal(tokens[2].value, 3.5);
});

test('ends the token stream with an eof marker', () => {
  const tokens = tokenize('true');
  assert.equal(tokens.at(-1).type, 'eof');
});

test('skips whitespace and keeps punctuation intact', () => {
  assert.deepEqual(values('  ( a , b )  '), ['(', 'a', ',', 'b', ')']);
  assert.deepEqual(types('a'), ['ident', 'eof']);
});

test('rejects characters outside the language', () => {
  assert.throws(
    () => tokenize('severity == high + 1'),
    (error) => error instanceof RuleError && error.code === 'E_LEX_UNEXPECTED_CHAR',
  );
  assert.throws(
    () => tokenize('a = b'),
    (error) => error instanceof RuleError && error.code === 'E_LEX_UNEXPECTED_CHAR',
  );
});

test('records the source position of a rejected character', () => {
  try {
    tokenize('abc +');
    assert.fail('expected a lexical error');
  } catch (error) {
    assert.equal(error.position, 4);
  }
});
