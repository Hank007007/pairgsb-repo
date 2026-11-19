const EFFECTS = new Set(['permit', 'deny']);

const OPERATORS = new Set(['eq', 'ne', 'lt', 'lte', 'gt', 'gte', 'in']);

export class PolicyFormatError extends Error {
  constructor(message, options) {
    super(message, options);
    this.name = 'PolicyFormatError';
  }
}

export function isPlainObject(value) {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function requireText(value, field, source) {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new PolicyFormatError(`${source}: field "${field}" must be a non-empty string`);
  }
  return value;
}

function normalizeCondition(condition, source) {
  if (condition === undefined || condition === null) {
    return undefined;
  }
  if (!isPlainObject(condition)) {
    throw new PolicyFormatError(`${source}: field "condition" must be an object`);
  }

  const normalized = {};

  for (const [key, spec] of Object.entries(condition)) {
    if (isPlainObject(spec)) {
      const operators = {};
      for (const [operator, expected] of Object.entries(spec)) {
        if (!OPERATORS.has(operator)) {
          throw new PolicyFormatError(`${source}: unsupported condition operator "${operator}"`);
        }
        if (operator === 'in') {
          if (!Array.isArray(expected) || expected.length === 0) {
            throw new PolicyFormatError(`${source}: operator "in" requires a non-empty array`);
          }
          operators[operator] = Object.freeze(expected.slice());
        } else {
          if (expected === undefined || expected === null) {
            throw new PolicyFormatError(`${source}: operator "${operator}" requires a value`);
          }
          operators[operator] = expected;
        }
      }
      if (Object.keys(operators).length === 0) {
        throw new PolicyFormatError(`${source}: condition for "${key}" has no operators`);
      }
      normalized[key] = Object.freeze(operators);
    } else {
      if (spec === undefined || spec === null) {
        throw new PolicyFormatError(`${source}: condition for "${key}" requires a value`);
      }
      normalized[key] = spec;
    }
  }

  return Object.freeze(normalized);
}

/**
 * Validate and freeze a single policy rule.
 *
 * @param {unknown} raw
 * @param {{ source?: string }} [options]
 * @returns {Readonly<object>}
 */
export function normalizeRule(raw, { source = 'policy' } = {}) {
  if (!isPlainObject(raw)) {
    throw new PolicyFormatError(`${source}: each rule must be an object`);
  }

  const id = requireText(raw.id, 'id', source);
  const principal = requireText(raw.principal, 'principal', `${source} (${id})`);
  const resource = requireText(raw.resource, 'resource', `${source} (${id})`);

  if (!Array.isArray(raw.action) || raw.action.length === 0) {
    throw new PolicyFormatError(`${source} (${id}): field "action" must be a non-empty array`);
  }
  const action = raw.action.map((entry) => requireText(entry, 'action[]', `${source} (${id})`));

  if (!EFFECTS.has(raw.effect)) {
    throw new PolicyFormatError(`${source} (${id}): field "effect" must be "permit" or "deny"`);
  }

  const condition = normalizeCondition(raw.condition, `${source} (${id})`);
  const description = raw.description === undefined ? '' : String(raw.description);

  return Object.freeze({
    id,
    description,
    principal,
    resource,
    action: Object.freeze(action),
    condition,
    effect: raw.effect,
  });
}

/**
 * Validate and freeze an ordered list of rules.
 *
 * @param {unknown} rawRules
 * @param {{ source?: string }} [options]
 * @returns {ReadonlyArray<Readonly<object>>}
 */
export function normalizeRules(rawRules, { source = 'policy' } = {}) {
  if (isPlainObject(rawRules) && Array.isArray(rawRules.rules)) {
    rawRules = rawRules.rules;
  }

  if (!Array.isArray(rawRules)) {
    throw new PolicyFormatError(`${source}: rule set must be an array or an object with a "rules" array`);
  }

  const seen = new Set();
  const rules = [];

  for (const raw of rawRules) {
    const rule = normalizeRule(raw, { source });
    if (seen.has(rule.id)) {
      throw new PolicyFormatError(`${source}: duplicate rule id "${rule.id}"`);
    }
    seen.add(rule.id);
    rules.push(rule);
  }

  return Object.freeze(rules);
}
