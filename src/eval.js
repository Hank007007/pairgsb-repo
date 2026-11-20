'use strict';

const { readField } = require('./fields');
const { hasFunction, FUNCTIONS, callFunction, isCidr } = require('./fns');
const { compileMatches } = require('./regex');

function truthy(value) {
  if (value === null || value === undefined || value === false) return false;
  if (typeof value === 'number') return value !== 0 && !Number.isNaN(value);
  if (typeof value === 'string') return value.length > 0;
  return true;
}

function evalNode(node, context) {
  switch (node.type) {
    case 'literal':
      return node.value;

    case 'array':
      return node.elements.map((element) => evalNode(element, context));

    case 'ident': {
      if (hasFunction(node.name)) return FUNCTIONS[node.name];
      const result = readField(context.event, [node.name], context.schema);
      return result.found ? result.value : null;
    }

    case 'field': {
      const result = readField(context.event, node.segments, context.schema);
      return result.found ? result.value : null;
    }

    case 'member': {
      const object = evalNode(node.object, context);
      if (object === null || object === undefined) return null;
      return object[node.property];
    }

    case 'unary':
      return !truthy(evalNode(node.operand, context));

    case 'logical': {
      const leftValue = evalNode(node.left, context);
      const rightValue = evalNode(node.right, context);
      return node.op === 'and'
        ? (leftValue && rightValue)
        : (leftValue || rightValue);
    }

    case 'binary':
      return evalComparison(node.op, evalNode(node.left, context), evalNode(node.right, context));

    case 'in': {
      const leftValue = evalNode(node.left, context);
      const rightValue = evalNode(node.right, context);
      return evalIn(leftValue, rightValue);
    }

    case 'matches': {
      const subject = evalNode(node.left, context);
      const pattern = evalNode(node.right, context);
      return evalMatches(node, subject, pattern, context);
    }

    case 'call': {
      const target = evalNode(node.callee, context);
      const args = node.args.map((argument) => evalNode(argument, context));
      if (typeof target !== 'function') return null;
      return target(...args);
    }

    default:
      return null;
  }
}

function evalComparison(op, left, right) {
  switch (op) {
    case '==':
      return left == right; // eslint-disable-line eqeqeq
    case '!=':
      return left != right; // eslint-disable-line eqeqeq
    case '<':
      return compareOrdered(left, right) < 0;
    case '<=':
      return compareOrdered(left, right) <= 0;
    case '>':
      return compareOrdered(left, right) > 0;
    case '>=':
      return compareOrdered(left, right) >= 0;
    default:
      return false;
  }
}

function compareOrdered(left, right) {
  const bothNumbers = typeof left === 'number' && typeof right === 'number';
  const bothStrings = typeof left === 'string' && typeof right === 'string';
  if (!bothNumbers && !bothStrings) return Number.NaN;
  if (left < right) return -1;
  if (left > right) return 1;
  return 0;
}

function evalIn(leftValue, rightValue) {
  if (Array.isArray(rightValue)) {
    return rightValue.some((candidate) => candidate === leftValue);
  }

  if (isCidr(rightValue)) {
    return cidrContains(rightValue, leftValue);
  }

  if (typeof rightValue === 'string') {
    return rightValue.includes(String(leftValue));
  }

  return false;
}

function cidrContains(block, value) {
  if (typeof value !== 'string') return false;
  const parts = value.trim().split('.');
  if (parts.length !== 4) return false;
  let address = 0;
  for (const part of parts) {
    if (!/^\d{1,3}$/.test(part)) return false;
    const octet = Number(part);
    if (octet > 255) return false;
    address = (address * 256) + octet;
  }
  return ((address & block.mask) >>> 0) === block.base;
}

function evalMatches(node, subject, patternValue, context) {
  if (typeof subject !== 'string') return false;

  const regexp = node.regexp || compileMatches(patternValue);
  return regexp.test(subject);
}

/**
 * Evaluate a compiled rule against a single event document. The return value
 * is the rule verdict: `true` when the expression is truthy, `false`
 * otherwise.
 */
function evaluate(compiled, event, options = {}) {
  const context = {
    event: event || {},
    schema: compiled.schema,
    options,
  };

  try {
    return truthy(evalNode(compiled.ast, context));
  } catch (error) {
    if (error instanceof RangeError) throw error;
    return false;
  }
}

module.exports = {
  evaluate,
  evalNode,
  truthy,
  callFunction,
};
