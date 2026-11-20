'use strict';

const { RuleError } = require('./errors');

const KEYWORDS = new Set(['and', 'or', 'not', 'in', 'matches', 'true', 'false', 'null']);
const COMPARISON_OPERATORS = new Set(['==', '!=', '<', '<=', '>', '>=']);

function isKeyword(value) {
  return KEYWORDS.has(value);
}

class Parser {
  constructor(tokens) {
    this.tokens = tokens;
    this.index = 0;
  }

  peek(offset = 0) {
    return this.tokens[Math.min(this.index + offset, this.tokens.length - 1)];
  }

  next() {
    const token = this.tokens[this.index];
    if (this.index < this.tokens.length - 1) this.index += 1;
    return token;
  }

  atEnd() {
    return this.peek().type === 'eof';
  }

  atKeyword(value) {
    const token = this.peek();
    return token.type === 'ident' && token.value === value;
  }

  atOperator(value) {
    const token = this.peek();
    return token.type === 'op' && token.value === value;
  }

  expectOperator(value) {
    const token = this.peek();
    if (token.type === 'op' && token.value === value) {
      return this.next();
    }
    if (token.type === 'eof') {
      throw new RuleError(
        'E_PARSE_UNEXPECTED_EOF',
        `Expected "${value}" but the rule ended`,
        token.start,
      );
    }
    throw new RuleError(
      'E_PARSE_UNEXPECTED_TOKEN',
      `Expected "${value}" but found ${describeToken(token)}`,
      token.start,
    );
  }

  parseProgram() {
    const expression = this.parseOr();
    if (!this.atEnd()) {
      const token = this.peek();
      throw new RuleError(
        'E_PARSE_TRAILING_INPUT',
        `Unexpected ${describeToken(token)} after end of expression`,
        token.start,
      );
    }
    return expression;
  }

  parseOr() {
    let left = this.parseAnd();
    while (this.atKeyword('or')) {
      this.next();
      const right = this.parseAnd();
      left = { type: 'logical', op: 'or', left, right, start: left.start };
    }
    return left;
  }

  parseAnd() {
    let left = this.parseNot();
    while (this.atKeyword('and')) {
      this.next();
      const right = this.parseNot();
      left = { type: 'logical', op: 'and', left, right, start: left.start };
    }
    return left;
  }

  parseNot() {
    if (this.atKeyword('not')) {
      const token = this.next();
      const operand = this.parseNot();
      return { type: 'unary', op: 'not', operand, start: token.start };
    }
    return this.parseComparison();
  }

  parseComparison() {
    const left = this.parsePostfix();

    const token = this.peek();
    if (token.type === 'op' && COMPARISON_OPERATORS.has(token.value)) {
      this.next();
      const right = this.parsePostfix();
      return { type: 'binary', op: token.value, left, right, start: left.start };
    }

    if (token.type === 'ident' && token.value === 'in') {
      this.next();
      const right = this.parsePostfix();
      return { type: 'in', left, right, start: left.start };
    }

    if (token.type === 'ident' && token.value === 'matches') {
      this.next();
      const right = this.parsePostfix();
      return { type: 'matches', left, right, start: left.start };
    }

    return left;
  }

  parsePostfix() {
    let expression = this.parsePrimary();

    for (;;) {
      if (this.atOperator('.')) {
        this.next();
        const segment = this.peek();
        if (segment.type !== 'ident') {
          throw new RuleError(
            'E_PARSE_UNEXPECTED_TOKEN',
            `Expected a field name after "." but found ${describeToken(segment)}`,
            segment.start,
          );
        }
        if (isKeyword(segment.value)) {
          throw new RuleError(
            'E_PARSE_UNEXPECTED_TOKEN',
            `"${segment.value}" is a keyword and cannot be used as a field name`,
            segment.start,
          );
        }
        this.next();
        expression = appendProperty(expression, segment.value);
        continue;
      }

      if (this.atOperator('(')) {
        expression = this.parseCall(expression);
        continue;
      }

      return expression;
    }
  }

  parsePrimary() {
    const token = this.peek();

    if (token.type === 'number') {
      this.next();
      return { type: 'literal', value: token.value, raw: token.raw, start: token.start };
    }

    if (token.type === 'string') {
      this.next();
      return { type: 'literal', value: token.value, start: token.start };
    }

    if (token.type === 'ident') {
      if (token.value === 'true' || token.value === 'false') {
        this.next();
        return { type: 'literal', value: token.value === 'true', start: token.start };
      }
      if (token.value === 'null') {
        this.next();
        return { type: 'literal', value: null, start: token.start };
      }
      if (isKeyword(token.value)) {
        throw new RuleError(
          'E_PARSE_UNEXPECTED_TOKEN',
          `Expected an expression but found keyword "${token.value}"`,
          token.start,
        );
      }
      this.next();
      return { type: 'ident', name: token.value, start: token.start };
    }

    if (token.type === 'op' && token.value === '(') {
      this.next();
      const expression = this.parseOr();
      this.expectOperator(')');
      return expression;
    }

    if (token.type === 'op' && token.value === '[') {
      return this.parseArray();
    }

    if (token.type === 'eof') {
      throw new RuleError('E_PARSE_UNEXPECTED_EOF', 'Unexpected end of rule', token.start);
    }

    throw new RuleError(
      'E_PARSE_UNEXPECTED_TOKEN',
      `Expected an expression but found ${describeToken(token)}`,
      token.start,
    );
  }

  parseCall(callee) {
    this.expectOperator('(');
    const args = [];
    if (!this.atOperator(')')) {
      args.push(this.parseOr());
      while (this.atOperator(',')) {
        this.next();
        args.push(this.parseOr());
      }
    }
    this.expectOperator(')');
    return { type: 'call', callee, args, start: callee.start };
  }

  parseArray() {
    const open = this.expectOperator('[');
    const elements = [];
    if (!this.atOperator(']')) {
      elements.push(this.parseOr());
      while (this.atOperator(',')) {
        this.next();
        elements.push(this.parseOr());
      }
    }
    this.expectOperator(']');
    return { type: 'array', elements, start: open.start };
  }
}

function appendProperty(expression, property) {
  if (expression.type === 'ident') {
    return { type: 'field', segments: [expression.name, property], start: expression.start };
  }
  if (expression.type === 'field') {
    return {
      type: 'field',
      segments: [...expression.segments, property],
      start: expression.start,
    };
  }
  return { type: 'member', object: expression, property, start: expression.start };
}

function describeToken(token) {
  if (token.type === 'eof') return 'end of rule';
  if (token.type === 'string') return 'a string literal';
  if (token.type === 'number') return `number ${token.raw}`;
  return JSON.stringify(token.value);
}

function parse(tokens) {
  return new Parser(tokens).parseProgram();
}

module.exports = {
  parse,
  KEYWORDS,
};
