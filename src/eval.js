'use strict';

const { readField, isForbiddenSegment } = require('./fields');
const { hasFunction, callFunction, isCidr } = require('./fns');
const { compileMatches } = require('./regex');

const MAX_STEPS = 10000;

class BudgetExceeded extends Error {}

function charge(context, amount = 1) {
  const budget = context.budget;
  if (!budget) return;
  budget.remaining -= amount;
  if (budget.remaining < 0) throw new BudgetExceeded('evaluation step budget exhausted');
}

function truthy(value) {
  if (value === null || value === undefined || value === false) return false;
  if (typeof value === 'number') return value !== 0 && !Number.isNaN(value);
  if (typeof value === 'string') return value.length > 0;
  return true;
}

function strictEqual(left, right) {
  const a = left === undefined ? null : left;
  const b = right === undefined ? null : right;
  if (a === null || b === null) return a === b;
  if (typeof a !== typeof b) return false;
  // Numbers, strings and booleans compare by value; arrays and CIDR blocks
  // compare by identity only.
  return a === b;
}

function evalNode(node, context) {
  if (context.budget === undefined || context.budget === null) {
    context.budget = { remaining: MAX_STEPS };
  }
  charge(context);

  switch (node.type) {
    case 'literal':
      return node.value;

    case 'array': {
      const values = [];
      for (const element of node.elements) {
        charge(context);
        values.push(evalNode(element, context));
      }
      return values;
    }

    case 'ident':
      return readFieldValue(context, [node.name]);

    case 'field':
      return readFieldValue(context, node.segments);

    case 'member': {
      const object = evalNode(node.object, context);
      return readMember(object, node.property);
    }

    case 'unary':
      return !truthy(evalNode(node.operand, context));

    case 'logical': {
      const leftValue = evalNode(node.left, context);
      if (node.op === 'and') {
        if (!truthy(leftValue)) return false;
        return truthy(evalNode(node.right, context));
      }
      if (truthy(leftValue)) return true;
      return truthy(evalNode(node.right, context));
    }

    case 'binary':
      return evalComparison(node.op, evalNode(node.left, context), evalNode(node.right, context));

    case 'in': {
      const leftValue = evalNode(node.left, context);
      const rightValue = evalNode(node.right, context);
      return evalIn(leftValue, rightValue, context);
    }

    case 'matches': {
      const subject = evalNode(node.left, context);
      const pattern = evalNode(node.right, context);
      return evalMatches(node, subject, pattern);
    }

    case 'call': {
      const callee = node.callee;
      if (!callee || callee.type !== 'ident' || !hasFunction(callee.name)) return null;
      const args = [];
      for (const argument of node.args) {
        charge(context);
        args.push(evalNode(argument, context));
      }
      return callFunction(callee.name, args);
    }

    default:
      return null;
  }
}

function readFieldValue(context, segments) {
  const result = readField(context.event, segments, context.schema);
  return result.found ? result.value : null;
}

function readMember(object, property) {
  if (object === null || typeof object !== 'object') return null;
  if (isForbiddenSegment(property)) return null;
  const prototype = Object.getPrototypeOf(object);
  if (prototype !== Object.prototype && prototype !== null) return null;
  if (!Object.prototype.hasOwnProperty.call(object, property)) return null;
  const value = object[property];
  return typeof value === 'function' ? null : value;
}

function evalComparison(op, left, right) {
  switch (op) {
    case '==':
      return strictEqual(left, right);
    case '!=':
      return !strictEqual(left, right);
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

function evalIn(leftValue, rightValue, context) {
  if (Array.isArray(rightValue)) {
    for (const candidate of rightValue) {
      charge(context);
      if (strictEqual(candidate, leftValue)) return true;
    }
    return false;
  }

  if (isCidr(rightValue)) {
    return cidrContains(rightValue, leftValue);
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

function evalMatches(node, subject, patternValue) {
  if (typeof subject !== 'string' || typeof patternValue !== 'string') return false;

  const regexp = node.regexp || compileMatches(patternValue);
  if (!regexp) return false;
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
    budget: { remaining: MAX_STEPS },
  };

  try {
    return truthy(evalNode(compiled.ast, context));
  } catch (error) {
    return false;
  }
}

module.exports = {
  evaluate,
  evalNode,
  truthy,
  callFunction,
  MAX_STEPS,
};
