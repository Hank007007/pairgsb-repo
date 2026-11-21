'use strict';

const { coerceScalar } = require('./coerce');

/**
 * Format a JSON Pointer path from tokens.
 *
 * @param {Array<string|number>} tokens
 * @returns {string}
 */
function toPointer(tokens) {
  if (tokens.length === 0) return '';
  return tokens
    .map((token) => '/' + String(token).replace(/~/g, '~0').replace(/\//g, '~1'))
    .join('');
}

/**
 * Push an error entry. When the plan node came from a `$ref`, the original
 * reference string is attached to the entry.
 *
 * @param {object[]} errors
 * @param {Array<string|number>} path
 * @param {string} keyword
 * @param {string} message
 * @param {object} [plan]
 */
function pushError(errors, path, keyword, message) {
  errors.push({ path: toPointer(path), keyword, message });
}

/**
 * Validate a scalar (non-object, non-array) value against a plan node.
 *
 * @param {object} plan
 * @param {unknown} value
 * @param {Array<string|number>} path
 * @param {object[]} errors
 */
function validateScalar(plan, value, path, errors) {
  if (plan.enum !== undefined && !plan.enum.includes(value)) {
    pushError(errors, path, 'enum', 'value is not one of the allowed values');
  }
  if (plan.const !== undefined && value !== plan.const) {
    pushError(errors, path, 'const', 'value does not equal the required constant');
  }

  if (plan.type === undefined) return;

  switch (plan.type) {
    case 'string':
      if (typeof value !== 'string') {
        pushError(errors, path, 'type', 'expected string');
        return;
      }
      if (plan.pattern) {
        if (!plan.pattern.regex.test(value)) {
          pushError(errors, path, 'pattern', 'value does not match pattern');
        }
      }
      break;
    case 'number':
      if (typeof value !== 'number' || !Number.isFinite(value)) {
        pushError(errors, path, 'type', 'expected number');
        return;
      }
      break;
    case 'integer':
      if (typeof value !== 'number' || !Number.isInteger(value)) {
        pushError(errors, path, 'type', 'expected integer');
        return;
      }
      break;
    case 'boolean':
      if (typeof value !== 'boolean') {
        pushError(errors, path, 'type', 'expected boolean');
        return;
      }
      break;
    case 'null':
      if (value !== null) {
        pushError(errors, path, 'type', 'expected null');
      }
      return;
    default:
      break;
  }

  if (typeof value === 'number') {
    if (plan.minimum !== undefined && value < plan.minimum) {
      pushError(errors, path, 'minimum', 'value is below minimum');
    }
    if (plan.maximum !== undefined && value > plan.maximum) {
      pushError(errors, path, 'maximum', 'value is above maximum');
    }
    if (plan.exclusiveMinimum !== undefined && value <= plan.exclusiveMinimum) {
      pushError(errors, path, 'exclusiveMinimum', 'value is not above exclusive minimum');
    }
    if (plan.exclusiveMaximum !== undefined && value >= plan.exclusiveMaximum) {
      pushError(errors, path, 'exclusiveMaximum', 'value is not below exclusive maximum');
    }
    if (plan.multipleOf !== undefined && plan.multipleOf !== 0) {
      if (value % plan.multipleOf !== 0) {
        pushError(errors, path, 'multipleOf', 'value is not a multiple');
      }
    }
  }
}

/**
 * Validate an instance against a compiled plan.
 *
 * @param {object} plan
 * @param {unknown} instance
 * @param {Array<string|number>} path
 * @param {object[]} errors
 * @param {number} depth
 */
function walk(plan, instance, path, errors, depth = 0) {
  if (!plan) return;

  if (plan.oneOf) {
    const matches = plan.oneOf.filter(
      (branch) => walkBranch(branch, instance, path) === 0,
    );
    if (matches.length !== 1) {
      pushError(errors, path, 'oneOf', 'value does not match exactly one branch');
    }
  }

  if (plan.anyOf) {
    const matches = plan.anyOf.filter(
      (branch) => walkBranch(branch, instance, path) === 0,
    );
    if (matches.length === 0) {
      pushError(errors, path, 'anyOf', 'value does not match any branch');
    }
  }

  if (plan.allOf) {
    for (const branch of plan.allOf) {
      walk(branch, instance, path, errors, depth);
    }
  }

  if (plan.type === 'object') {
    if (instance === null || typeof instance !== 'object' || Array.isArray(instance)) {
      pushError(errors, path, 'type', 'expected object');
      return;
    }

    for (const name of plan.required) {
      if (!(name in instance)) {
        pushError(errors, [...path, name], 'required', 'required property is missing');
      }
    }

    for (const [name, child] of Object.entries(plan.properties)) {
      if (name in instance) {
        walk(child, instance[name], [...path, name], errors, depth + 1);
      }
    }

    for (const name of Object.keys(instance)) {
      if (plan.properties[name] !== undefined) continue;

      if (plan.additionalProperties === false) {
        pushError(errors, [...path, name], 'additionalProperties', 'unknown property');
      } else if (plan.additionalProperties === undefined) {
        // No explicit policy: unknown properties are accepted.
      } else if (plan.additionalProperties && plan.additionalProperties !== true) {
        walk(plan.additionalProperties, instance[name], [...path, name], errors, depth + 1);
      }

      for (const entry of plan.patternProperties) {
        if (entry.regex.test(name)) {
          walk(entry.plan, instance[name], [...path, name], errors, depth + 1);
        }
      }
    }
  } else if (plan.type === 'array') {
    if (!Array.isArray(instance)) {
      pushError(errors, path, 'type', 'expected array');
      return;
    }
    if (plan.minItems !== undefined && instance.length < plan.minItems) {
      pushError(errors, path, 'minItems', 'array is too short');
    }
    if (plan.maxItems !== undefined && instance.length > plan.maxItems) {
      pushError(errors, path, 'maxItems', 'array is too long');
    }
    if (plan.items) {
      for (let i = 0; i < instance.length; i += 1) {
        walk(plan.items, instance[i], [...path, i], errors, depth + 1);
      }
    }
  } else {
    validateScalar(plan, instance, path, errors);
  }
}

/**
 * Validate one combinator branch. Returns the number of errors produced.
 *
 * @param {object} branch
 * @param {unknown} instance
 * @param {Array<string|number>} path
 * @returns {number}
 */
function walkBranch(branch, instance, path) {
  const branchErrors = [];
  walk(branch, instance, path, branchErrors);
  return branchErrors.length;
}

module.exports = {
  walk,
  toPointer,
  pushError,
};