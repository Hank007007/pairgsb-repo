'use strict';

const { tokenize } = require('./lexer');
const { parse } = require('./parser');
const { getFieldSchema } = require('./fields');
const { compileMatches } = require('./regex');

const cache = new Map();

function annotate(node) {
  switch (node.type) {
    case 'matches':
      annotate(node.left);
      annotate(node.right);
      if (node.right.type === 'literal' && typeof node.right.value === 'string') {
        node.regexp = compileMatches(node.right.value);
      }
      return node;

    case 'binary':
    case 'in':
      annotate(node.left);
      annotate(node.right);
      return node;

    case 'logical':
      annotate(node.left);
      annotate(node.right);
      return node;

    case 'unary':
      annotate(node.operand);
      return node;

    case 'member':
      annotate(node.object);
      return node;

    case 'call':
      annotate(node.callee);
      for (const argument of node.args) annotate(argument);
      return node;

    case 'array':
      for (const element of node.elements) annotate(element);
      return node;

    default:
      return node;
  }
}

function build(source, tenantId) {
  const tokens = tokenize(source);
  const ast = parse(tokens);
  annotate(ast);

  return Object.freeze({
    source,
    tenantId,
    schema: getFieldSchema(tenantId),
    ast,
  });
}

/**
 * Compile a rule expression for a tenant. Compiled rules are cached so repeated
 * events do not re-parse the same text.
 */
function compileRule(source, options = {}) {
  const tenantId = options.tenantId || 'default';
  const key = source;

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
