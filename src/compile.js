'use strict';

const { tokenize } = require('./lexer');
const { parse } = require('./parser');
const { getFieldSchema, isForbiddenSegment } = require('./fields');
const { hasFunction } = require('./fns');
const { parsePattern } = require('./regex');
const { RuleError } = require('./errors');

const cache = new Map();

const ARITY = {
  lower: [1, 1],
  upper: [1, 1],
  trim: [1, 1],
  len: [1, 1],
  contains: [2, 2],
  startsWith: [2, 2],
  endsWith: [2, 2],
  coalesce: [1, Infinity],
  int: [1, 1],
  cidr: [1, 1],
  isCidr: [1, 1],
};

function validateFieldPath(node, schema) {
  for (const segment of node.segments) {
    if (isForbiddenSegment(segment)) {
      throw new RuleError(
        'E_COMPILE_UNKNOWN_FIELD',
        `Field path segment "${segment}" is not allowed`,
        node.start,
      );
    }
  }
  const path = node.segments.join('.');
  if (!schema.fields.has(path)) {
    throw new RuleError(
      'E_COMPILE_UNKNOWN_FIELD',
      `Field path "${path}" is not declared for this tenant`,
      node.start,
    );
  }
}

function validateCall(node) {
  const callee = node.callee;
  if (callee.type !== 'ident' || !hasFunction(callee.name)) {
    const name = callee.type === 'ident' ? callee.name : 'expression';
    throw new RuleError(
      'E_COMPILE_UNKNOWN_FUNCTION',
      `"${name}" is not a whitelisted function`,
      node.start,
    );
  }
  const [min, max] = ARITY[callee.name];
  if (node.args.length < min || node.args.length > max) {
    throw new RuleError(
      'E_COMPILE_BAD_ARITY',
      `"${callee.name}" does not take ${node.args.length} argument(s)`,
      node.start,
    );
  }
}

function validate(node, schema) {
  switch (node.type) {
    case 'matches':
      validate(node.left, schema);
      validate(node.right, schema);
      if (node.right.type === 'literal' && typeof node.right.value === 'string') {
        const { flags, body } = parsePattern(node.right.value);
        node.regexp = new RegExp(body, flags);
      }
      return node;

    case 'field':
      validateFieldPath(node, schema);
      return node;

    case 'ident':
      if (!schema.fields.has(node.name)) {
        throw new RuleError(
          'E_COMPILE_UNKNOWN_FIELD',
          `Field "${node.name}" is not declared for this tenant`,
          node.start,
        );
      }
      return node;

    case 'call':
      // A call through a field path is never allowed, but report an
      // undeclared or forbidden path first when it is also invalid.
      if (node.callee.type === 'field') validateFieldPath(node.callee, schema);
      else if (node.callee.type !== 'ident') validate(node.callee, schema);
      validateCall(node);
      for (const argument of node.args) validate(argument, schema);
      return node;

    case 'binary':
    case 'in':
      validate(node.left, schema);
      validate(node.right, schema);
      return node;

    case 'logical':
      validate(node.left, schema);
      validate(node.right, schema);
      return node;

    case 'unary':
      validate(node.operand, schema);
      return node;

    case 'member':
      validate(node.object, schema);
      return node;

    case 'array':
      for (const element of node.elements) validate(element, schema);
      return node;

    default:
      return node;
  }
}

function build(source, tenantId) {
  const tokens = tokenize(source);
  const ast = parse(tokens);
  const schema = getFieldSchema(tenantId);
  validate(ast, schema);

  return Object.freeze({
    source,
    tenantId,
    schema,
    ast,
  });
}

/**
 * Compile a rule expression for a tenant. Compiled rules are cached so repeated
 * events do not re-parse the same text.
 */
function compileRule(source, options = {}) {
  const tenantId = options.tenantId || 'default';
  const key = JSON.stringify([tenantId, source]);

  const cached = cache.get(key);
  if (cached) return cached;

  const compiled = build(source, tenantId);
  cache.set(key, compiled);
  return compiled;
}

function clearRuleCache() {
  cache.clear();
}

module.exports = {
  compileRule,
  clearRuleCache,
};
