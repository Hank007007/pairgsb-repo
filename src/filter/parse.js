'use strict';

const { badRequest } = require('../util/errors');

const OPERATORS = new Set(['eq', 'ne', 'co', 'sw', 'ew', 'gt', 'ge', 'lt', 'le']);
const PR = 'pr';

function readQuoted(text, start) {
  const quote = text[start];
  let i = start + 1;
  while (i < text.length) {
    const ch = text[i];
    if (ch === '\\') {
      if (i + 1 >= text.length) {
        throw badRequest('invalidFilter', 'unterminated string literal');
      }
      i += 2;
      continue;
    }
    if (ch === quote) {
      return { raw: text.slice(start, i + 1), next: i + 1 };
    }
    i += 1;
  }
  throw badRequest('invalidFilter', 'unterminated string literal');
}

function tokenize(input) {
  const tokens = [];
  const text = String(input);
  let i = 0;
  while (i < text.length) {
    const ch = text[i];
    if (/\s/.test(ch)) {
      i += 1;
      continue;
    }
    if (ch === '(' || ch === ')' || ch === '[' || ch === ']') {
      tokens.push({ type: ch, value: ch, pos: i });
      i += 1;
      continue;
    }
    if (ch === '"' || ch === "'") {
      const literal = readQuoted(text, i);
      tokens.push({ type: 'string', value: literal.raw, pos: i });
      i = literal.next;
      continue;
    }
    let j = i;
    while (j < text.length && !/[\s()[\]]/.test(text[j])) {
      j += 1;
    }
    tokens.push({ type: 'word', value: text.slice(i, j), pos: i });
    i = j;
  }
  return tokens;
}

function parseValue(raw) {
  if (raw.startsWith('"')) {
    try {
      return JSON.parse(raw);
    } catch (err) {
      throw badRequest('invalidFilter', 'malformed string literal');
    }
  }
  if (raw.startsWith("'")) {
    return raw.slice(1, -1).replace(/\\(['\\])/g, '$1');
  }
  if (/^(true|false)$/i.test(raw)) {
    return raw.toLowerCase() === 'true';
  }
  if (/^null$/i.test(raw)) {
    return null;
  }
  if (/^-?\d+(\.\d+)?$/.test(raw)) {
    return Number(raw);
  }
  return raw;
}

function parseAttrPath(text) {
  const trimmed = String(text).trim();
  if (trimmed.length === 0) {
    throw badRequest('invalidFilter', 'empty attribute path');
  }
  const bracket = trimmed.indexOf('[');
  if (bracket === -1) {
    return { attr: trimmed, filter: null };
  }
  const attr = trimmed.slice(0, bracket).trim();
  const rest = trimmed.slice(bracket);
  const inner = /^\[(.*)\]$/.exec(rest);
  if (!inner) {
    throw badRequest('invalidFilter', `malformed value filter in ${trimmed}`);
  }
  return { attr, filter: inner[1] };
}

class Parser {
  constructor(tokens, source) {
    this.tokens = tokens;
    this.index = 0;
    this.source = source;
  }

  peek() {
    return this.tokens[this.index] || null;
  }

  next() {
    const token = this.tokens[this.index];
    this.index += 1;
    return token || null;
  }

  atWord(word) {
    const token = this.peek();
    return token && token.type === 'word' && token.value.toLowerCase() === word;
  }

  parse() {
    if (this.tokens.length === 0) {
      throw badRequest('invalidFilter', 'empty filter');
    }
    const expr = this.parseOr();
    if (this.index !== this.tokens.length) {
      throw badRequest('invalidFilter', 'trailing tokens in filter');
    }
    return expr;
  }

  parseOr() {
    let left = this.parseAnd();
    while (this.atWord('or')) {
      this.next();
      const right = this.parseAnd();
      left = { type: 'or', left, right };
    }
    return left;
  }

  parseAnd() {
    let left = this.parseNot();
    while (this.atWord('and')) {
      this.next();
      const right = this.parseNot();
      left = { type: 'and', left, right };
    }
    return left;
  }

  parseNot() {
    if (this.atWord('not')) {
      this.next();
      return { type: 'not', expr: this.parseNot() };
    }
    return this.parsePrimary();
  }

  parsePrimary() {
    const token = this.peek();
    if (!token) {
      throw badRequest('invalidFilter', 'unexpected end of filter');
    }
    if (token.type === '(') {
      this.next();
      const expr = this.parseOr();
      const close = this.next();
      if (!close || close.type !== ')') {
        throw badRequest('invalidFilter', 'missing closing parenthesis');
      }
      return expr;
    }
    return this.parseComparison();
  }

  parseComparison() {
    const attrToken = this.next();
    if (!attrToken || (attrToken.type !== 'word' && attrToken.type !== 'string')) {
      throw badRequest('invalidFilter', 'expected attribute name');
    }
    let pathText = String(attrToken.value);
    if (this.peek() && this.peek().type === '[') {
      let depth = 0;
      let closePos = null;
      while (this.peek()) {
        const token = this.next();
        if (token.type === '[') depth += 1;
        if (token.type === ']') {
          depth -= 1;
          if (depth === 0) {
            closePos = token.pos;
            break;
          }
        }
      }
      if (closePos === null) {
        throw badRequest('invalidFilter', 'unbalanced brackets in attribute path');
      }
      pathText = this.source.slice(attrToken.pos, closePos + 1);
    }

    const opToken = this.next();
    if (!opToken || opToken.type !== 'word') {
      throw badRequest('invalidFilter', 'expected operator');
    }
    const op = opToken.value.toLowerCase();
    if (op === PR) {
      return { type: 'pr', path: parseAttrPath(pathText) };
    }
    if (!OPERATORS.has(op)) {
      throw badRequest('invalidFilter', `unknown operator ${opToken.value}`);
    }
    const valueToken = this.next();
    if (!valueToken) {
      throw badRequest('invalidFilter', 'missing comparison value');
    }
    if (valueToken.type !== 'string' && valueToken.type !== 'word') {
      throw badRequest('invalidFilter', 'invalid comparison value');
    }
    const value = parseValue(String(valueToken.value));
    return { type: 'cmp', path: parseAttrPath(pathText), op, value };
  }
}

function parse(input) {
  if (input === null || input === undefined) {
    throw badRequest('invalidFilter', 'missing filter');
  }
  const source = String(input);
  return new Parser(tokenize(source), source).parse();
}

module.exports = { parse, parseValue, parseAttrPath, tokenize };
