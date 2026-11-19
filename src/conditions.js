const OPERATORS = new Set(['eq', 'ne', 'lt', 'lte', 'gt', 'gte', 'in']);

function isMissing(context, key) {
  return !Object.prototype.hasOwnProperty.call(context, key);
}

function isNumeric(value) {
  if (typeof value === 'number') {
    return Number.isFinite(value);
  }
  return (
    typeof value === 'string' &&
    value.trim() !== '' &&
    Number.isFinite(Number(value))
  );
}

function looseEqual(actual, expected) {
  if (isNumeric(actual) && isNumeric(expected)) {
    return Number(actual) === Number(expected);
  }
  return actual === expected;
}

function compareOrdering(actual, expected) {
  if (isNumeric(actual) && isNumeric(expected)) {
    const left = Number(actual);
    const right = Number(expected);
    return left < right ? -1 : left > right ? 1 : 0;
  }

  if (typeof actual === 'string' && typeof expected === 'string') {
    return actual < expected ? -1 : actual > expected ? 1 : 0;
  }

  if (typeof actual === 'boolean' && typeof expected === 'boolean') {
    const left = actual ? 1 : 0;
    const right = expected ? 1 : 0;
    return left < right ? -1 : left > right ? 1 : 0;
  }

  return null;
}

function compare(operator, actual, expected) {
  if (operator === 'eq') {
    return looseEqual(actual, expected);
  }
  if (operator === 'ne') {
    return !looseEqual(actual, expected);
  }
  if (operator === 'in') {
    return Array.isArray(expected) && expected.some((entry) => looseEqual(actual, entry));
  }

  const ordering = compareOrdering(actual, expected);
  if (ordering === null) {
    return false;
  }

  switch (operator) {
    case 'lt':
      return ordering < 0;
    case 'lte':
      return ordering <= 0;
    case 'gt':
      return ordering > 0;
    case 'gte':
      return ordering >= 0;
    default:
      return false;
  }
}

/**
 * Evaluate a rule condition against a request context.
 *
 * A condition is an object whose keys name context fields. Each value is
 * either a scalar (shorthand for `eq`) or an object of comparison operators.
 *
 * @param {Record<string, unknown> | null | undefined} condition
 * @param {Record<string, unknown>} context
 * @returns {boolean}
 */
export function evaluate(condition, context) {
  if (condition === undefined || condition === null) {
    return true;
  }
  if (context === undefined || context === null) {
    context = {};
  }

  for (const [key, spec] of Object.entries(condition)) {
    if (isMissing(context, key)) {
      return false;
    }

    const actual = context[key];

    if (spec === null || typeof spec !== 'object' || Array.isArray(spec)) {
      if (!compare('eq', actual, spec)) {
        return false;
      }
      continue;
    }

    for (const [operator, expected] of Object.entries(spec)) {
      if (!OPERATORS.has(operator)) {
        return false;
      }
      if (!compare(operator, actual, expected)) {
        return false;
      }
    }
  }

  return true;
}
