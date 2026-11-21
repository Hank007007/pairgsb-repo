'use strict';

/**
 * Type coercion and default application.
 *
 * Coercion runs before validation. `applyDefaults` runs after coercion and
 * before validation as well, so that defaults participate in validation like
 * any other field.
 */

/**
 * Coerce a scalar value toward a target `type`.
 *
 * @param {unknown} value
 * @param {string|undefined} type
 * @returns {unknown}
 */
function coerceScalar(value, type) {
  if (typeof value !== 'string') {
    return value;
  }

  if (type === 'integer') {
    const n = parseInt(value, 10);
    if (!Number.isNaN(n)) return n;
    return value;
  }

  if (type === 'number') {
    const n = parseFloat(value);
    if (!Number.isNaN(n)) return n;
    return value;
  }

  if (type === 'boolean') {
    if (value === 'true') return true;
    if (value === 'false') return false;
    return Boolean(value);
  }

  if (type === 'array') {
    return [value];
  }

  return value;
}

/**
 * Walk the instance alongside a compiled plan, coercing scalars in place.
 *
 * @param {object} plan
 * @param {unknown} instance
 * @returns {unknown}
 */
function coerceInstance(plan, instance) {
  if (instance === null || instance === undefined) return instance;

  if (typeof instance === 'object' && !Array.isArray(instance)) {
    if (!plan.properties) return instance;
    for (const [name, child] of Object.entries(plan.properties)) {
      if (instance[name] !== undefined) {
        instance[name] = coerceInstance(child, instance[name]);
      }
    }
    if (plan.additionalProperties && typeof plan.additionalProperties === 'object') {
      for (const name of Object.keys(instance)) {
        if (plan.properties[name] === undefined) {
          instance[name] = coerceInstance(plan.additionalProperties, instance[name]);
        }
      }
    }
    return instance;
  }

  if (Array.isArray(instance)) {
    if (!plan.items) return instance;
    for (let i = 0; i < instance.length; i += 1) {
      instance[i] = coerceInstance(plan.items, instance[i]);
    }
    return instance;
  }

  return coerceScalar(instance, plan.type);
}

/**
 * Apply schema defaults to an instance. Missing properties declared in the
 * plan gain the default value.
 *
 * @param {object} plan
 * @param {unknown} instance
 * @returns {unknown}
 */
function applyDefaults(plan, instance) {
  if (instance === null || instance === undefined) {
    // A missing object can be materialised when it has defaults to fill.
    if (plan && plan.properties) {
      instance = {};
    } else {
      return instance;
    }
  }

  if (typeof instance !== 'object' || Array.isArray(instance)) {
    return instance;
  }

  if (!plan || !plan.properties) {
    return instance;
  }

  for (const [name, child] of Object.entries(plan.properties)) {
    if (instance[name] === undefined) {
      if (child.default !== undefined) {
        instance[name] = child.default;
      }
    } else {
      applyDefaults(child, instance[name]);
    }
  }

  if (plan.default !== undefined && Object.keys(instance).length === 0) {
    for (const [key, value] of Object.entries(plan.default)) {
      if (instance[key] === undefined) {
        instance[key] = value;
      }
    }
  }

  return instance;
}

module.exports = {
  coerceScalar,
  coerceInstance,
  applyDefaults,
};