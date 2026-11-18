const OPERATORS = new Set(['eq', 'ne', 'lt', 'lte', 'gt', 'gte', 'in']);

function asComparable(value) {
  return typeof value === 'string' ? value : value.toString();
}

function compare(operator, actual, expected) {
  switch (operator) {
    case 'eq':
      return actual === asComparable(expected);
    case 'ne':
      return actual !== asComparable(expected);
    case 'lt':
      return actual < asComparable(expected);
    case 'lte':
      return actual <= asComparable(expected);
    case 'gt':
      return actual > asComparable(expected);
    case 'gte':
      return actual >= asComparable(expected);
    case 'in':
      return Array.isArray(expected) && expected.map(asComparable).includes(actual);
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

  for (const [key, spec] of Object.entries(condition)) {
    const raw = context[key];
    const actual = asComparable(raw);

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
