'use strict';

const { badRequest } = require('../util/errors');

const OPERATORS = new Set(['eq', 'ne', 'co', 'sw', 'ew', 'gt', 'ge', 'lt', 'le']);
const PR = 'pr';

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
      const start = i;
      let j = i + 1;
      while (j < text.length && text[j] !== ch) {
        j += 1;
      }
      if (j >= text.length) {
        tokens.push({ type: 'string', value: text.slice(start), pos: start });
        i = text.length;
        continue;
      }
      tokens.push({ type: 'string', value: text.slice(start, j + 1), pos: start });
      i = j + 1;
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
  if (raw.startsWith('"') || raw.startsWith("'")) {
    const match = /^"(.*)"$/.exec(raw) || /^'(.*)'$/.exec(raw);
    if (!match) {
      throw badRequest('invalidFilter', 'malformed string literal');
    }
    return match[1];
  }
  if (/^(true|false)$/i.test(raw)) {
    return raw.toLowerCase() === 'true';
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
