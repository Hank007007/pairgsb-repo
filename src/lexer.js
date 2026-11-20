'use strict';

const { RuleError } = require('./errors');

const TWO_CHAR_OPERATORS = new Set(['==', '!=', '<=', '>=']);
const ONE_CHAR_TOKENS = new Set(['(', ')', '[', ']', ',', '.', '<', '>']);

function isDigit(ch) {
  return ch >= '0' && ch <= '9';
}

function isIdentStart(ch) {
  return (ch >= 'a' && ch <= 'z')
    || (ch >= 'A' && ch <= 'Z')
    || ch === '_'
    || ch === '$'
    || ch === '#'
    || ch.charCodeAt(0) > 0x7f;
}

function isIdentPart(ch) {
  return isIdentStart(ch) || isDigit(ch);
}

function readNumber(source, start) {
  let i = start;
  while (i < source.length && isDigit(source[i])) i += 1;
  if (source[i] === '.' && isDigit(source[i + 1])) {
    i += 1;
    while (i < source.length && isDigit(source[i])) i += 1;
  }
  const raw = source.slice(start, i);
  return {
    token: { type: 'number', value: Number(raw), raw, start, end: i },
    next: i,
  };
}

function readIdent(source, start) {
  let i = start + 1;
  while (i < source.length && isIdentPart(source[i])) i += 1;
  return {
    token: { type: 'ident', value: source.slice(start, i), start, end: i },
    next: i,
  };
}

function readString(source, start) {
  let i = start + 1;
  let value = '';
  while (i < source.length && source[i] !== '"') {
    value += source[i];
    i += 1;
  }
  if (i < source.length) i += 1;
  return {
    token: { type: 'string', value, start, end: i },
    next: i,
  };
}

function tokenize(source) {
  if (typeof source !== 'string') {
    throw new TypeError('rule source must be a string');
  }

  const tokens = [];
  let i = 0;

  while (i < source.length) {
    const ch = source[i];

    if (ch === ' ' || ch === '\t' || ch === '\r' || ch === '\n') {
      i += 1;
      continue;
    }

    if (ch === '"') {
      const result = readString(source, i);
      tokens.push(result.token);
      i = result.next;
      continue;
    }

    if (isDigit(ch)) {
      const result = readNumber(source, i);
      tokens.push(result.token);
      i = result.next;
      continue;
    }

    if (isIdentStart(ch)) {
      const result = readIdent(source, i);
      tokens.push(result.token);
      i = result.next;
      continue;
    }

    const pair = source.slice(i, i + 2);
    if (TWO_CHAR_OPERATORS.has(pair)) {
      tokens.push({ type: 'op', value: pair, start: i, end: i + 2 });
      i += 2;
      continue;
    }

    if (ONE_CHAR_TOKENS.has(ch)) {
      tokens.push({ type: 'op', value: ch, start: i, end: i + 1 });
      i += 1;
      continue;
    }

    throw new RuleError('E_LEX_UNEXPECTED_CHAR', `Unexpected character ${JSON.stringify(ch)}`, i);
  }

  tokens.push({ type: 'eof', value: null, start: source.length, end: source.length });
  return tokens;
}

module.exports = {
  tokenize,
};
