'use strict';

const { badRequest } = require('../util/errors');
const { parse } = require('./parse');

function tenantScope(tenant) {
  return { kind: 'cmp', path: 'tenant', op: 'eq', value: tenant };
}

function escapeLike(text) {
  return String(text).replace(/[\\%_]/g, (ch) => `\\${ch}`);
}

function likePattern(op, value) {
  const text = escapeLike(value);
  if (op === 'co') return `%${text}%`;
  if (op === 'sw') return `${text}%`;
  if (op === 'ew') return `%${text}`;
  return text;
}

function compileComparison(node) {
  const attr = node.path.attr;
  if (node.path.filter !== null) {
    return {
      kind: 'element',
      path: attr,
      condition: compileSubtree(parse(node.path.filter)),
    };
  }
  if (node.op === 'co' || node.op === 'sw' || node.op === 'ew') {
    return { kind: 'like', path: attr, pattern: likePattern(node.op, node.value) };
  }
  return { kind: 'cmp', path: attr, op: node.op, value: node.value };
}

function compileSubtree(node) {
  switch (node.type) {
    case 'and':
      return {
        kind: 'and',
        items: [compileSubtree(node.left), compileSubtree(node.right)],
      };
    case 'or':
      return {
        kind: 'or',
        items: [compileSubtree(node.left), compileSubtree(node.right)],
      };
    case 'not':
      return { kind: 'not', item: compileSubtree(node.expr) };
    case 'cmp':
      return compileComparison(node);
    case 'pr':
      return { kind: 'pr', path: node.path.attr };
    default:
      throw badRequest('invalidFilter', 'unsupported expression');
  }
}

function compile(ast, tenant) {
  if (typeof tenant !== 'string' || tenant.length === 0) {
    throw badRequest('invalidFilter', 'missing tenant');
  }
  const client = compileSubtree(ast);
  return { kind: 'and', items: [tenantScope(tenant), client] };
}

module.exports = { compile, compileSubtree, tenantScope, likePattern };
