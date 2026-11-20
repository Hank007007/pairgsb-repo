'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { tokenize } = require('../src/lexer');
const { parse } = require('../src/parser');
const { RuleError } = require('../src/errors');

function ast(source) {
  return parse(tokenize(source));
}

test('parses a field comparison', () => {
  const node = ast('severity == "high"');
  assert.equal(node.type, 'binary');
  assert.equal(node.op, '==');
  assert.deepEqual(node.left, { type: 'ident', name: 'severity', start: 0 });
  assert.deepEqual(node.right, { type: 'literal', value: 'high', start: 12 });
});

test('parses dotted field paths into segments', () => {
  const node = ast('source.geo.country == "US"');
  assert.equal(node.type, 'binary');
  assert.deepEqual(node.left.segments, ['source', 'geo', 'country']);
  assert.equal(node.left.type, 'field');
});

test('gives and higher precedence than or', () => {
  const node = ast('a or b and c');
  assert.equal(node.type, 'logical');
  assert.equal(node.op, 'or');
  assert.equal(node.left.name, 'a');
  assert.equal(node.right.type, 'logical');
  assert.equal(node.right.op, 'and');
});

test('not binds tighter than and', () => {
  const node = ast('not a and b');
  assert.equal(node.type, 'logical');
  assert.equal(node.op, 'and');
  assert.equal(node.left.type, 'unary');
  assert.equal(node.left.op, 'not');
});

test('parses calls with arguments', () => {
  const node = ast('contains(message, "fail")');
  assert.equal(node.type, 'call');
  assert.equal(node.callee.name, 'contains');
  assert.equal(node.args.length, 2);
  assert.equal(node.args[1].value, 'fail');
});

test('parses array literals', () => {
  const node = ast('event.action in ["login", "logout"]');
  assert.equal(node.type, 'in');
  assert.equal(node.right.type, 'array');
  assert.deepEqual(node.right.elements.map((element) => element.value), ['login', 'logout']);
});

test('parses matches at comparison precedence', () => {
  const node = ast('message matches "login"');
  assert.equal(node.type, 'matches');
  assert.equal(node.left.name, 'message');
  assert.equal(node.right.value, 'login');
});

test('parses parenthesised expressions', () => {
  const node = ast('(a or b) and c');
  assert.equal(node.type, 'logical');
  assert.equal(node.op, 'and');
  assert.equal(node.left.type, 'logical');
  assert.equal(node.left.op, 'or');
});

test('reports an incomplete expression', () => {
  assert.throws(
    () => ast('severity =='),
    (error) => error instanceof RuleError && error.code === 'E_PARSE_UNEXPECTED_EOF',
  );
});

test('reports input left after a complete expression', () => {
  assert.throws(
    () => ast('1 2'),
    (error) => error instanceof RuleError && error.code === 'E_PARSE_TRAILING_INPUT',
  );
});

test('reports a missing closing parenthesis', () => {
  assert.throws(
    () => ast('(1 == 1'),
    (error) => error instanceof RuleError && error.code === 'E_PARSE_UNEXPECTED_EOF',
  );
});

test('rejects keywords used as field names', () => {
  assert.throws(
    () => ast('source.and == 1'),
    (error) => error instanceof RuleError && error.code === 'E_PARSE_UNEXPECTED_TOKEN',
  );
});
