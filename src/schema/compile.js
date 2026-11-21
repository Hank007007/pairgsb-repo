'use strict';

const { SchemaError } = require('./errors');
const { resolveRef } = require('./refs');
const limits = require('../policy/limits');

/**
 * Compiled regular expressions, shared for the lifetime of the process.
 * @type {Map<string, RegExp>}
 */
const patternCache = new Map();

const SUPPORTED_TYPES = new Set([
  'object',
  'array',
  'string',
  'number',
  'integer',
  'boolean',
  'null',
]);

const COMBINATORS = new Set(['oneOf', 'anyOf', 'allOf']);

/**
 * Compile a `pattern`/`patternFlags` pair into a RegExp.
 *
 * @param {object} node
 * @returns {{source: string, flags: string, regex: RegExp}}
 */
function compilePattern(node) {
  const source = node.pattern;
  const flags = typeof node.patternFlags === 'string' ? node.patternFlags : '';

  const cached = patternCache.get(source);
  if (cached) {
    return { source, flags, regex: cached };
  }

  const regex = new RegExp(source, flags);
  patternCache.set(source, regex);
  return { source, flags, regex };
}

/**
 * Compile a single schema node into a validation-plan node.
 *
 * @param {unknown} node
 * @param {object} ctx
 * @param {number} depth
 * @returns {Promise<object>}
 */
async function compileNode(node, ctx, depth) {
  if (node === null || typeof node !== 'object' || Array.isArray(node)) {
    throw new SchemaError('schema node must be an object', {
      code: 'SCHEMA_SHAPE',
    });
  }

  if (typeof node.$ref === 'string') {
    const target = await resolveRef(ctx.root, node.$ref, {
      visited: ctx.visited,
      fetcher: ctx.fetcher,
    });
    const plan = await compileNode(target, ctx, depth + 1);
    return { ...plan, ref: node.$ref };
  }

  const plan = {
    type: node.type,
    title: node.title,
    description: node.description,
    nullable: node.type === 'null',
  };

  if (node.type !== undefined) {
    if (!SUPPORTED_TYPES.has(node.type)) {
      throw new SchemaError(`unsupported type: ${String(node.type)}`, {
        code: 'SCHEMA_TYPE',
      });
    }
  }

  if (node.default !== undefined) plan.default = node.default;

  if (typeof node.format === 'string') plan.format = node.format;

  if (node.enum !== undefined) plan.enum = node.enum;
  if (node.const !== undefined) plan.const = node.const;

  if (typeof node.pattern === 'string') {
    plan.pattern = compilePattern(node);
  }

  for (const keyword of ['minimum', 'maximum', 'exclusiveMinimum', 'exclusiveMaximum', 'multipleOf']) {
    if (node[keyword] !== undefined) plan[keyword] = node[keyword];
  }

  if (node.type === 'object') {
    plan.properties = Object.create(null);
    if (node.properties && typeof node.properties === 'object') {
      for (const [name, child] of Object.entries(node.properties)) {
        plan.properties[name] = await compileNode(child, ctx, depth + 1);
      }
    }

    plan.patternProperties = [];
    if (node.patternProperties && typeof node.patternProperties === 'object') {
      for (const [source, child] of Object.entries(node.patternProperties)) {
        const compiled = compilePattern({ pattern: source });
        plan.patternProperties.push({
          source,
          regex: compiled.regex,
          plan: await compileNode(child, ctx, depth + 1),
        });
      }
    }

    if (node.additionalProperties === undefined) {
      plan.additionalProperties = undefined;
    } else if (node.additionalProperties === false || node.additionalProperties === true) {
      plan.additionalProperties = node.additionalProperties;
    } else {
      plan.additionalProperties = await compileNode(
        node.additionalProperties,
        ctx,
        depth + 1,
      );
    }

    plan.required = Array.isArray(node.required) ? node.required.slice() : [];
    plan.minProperties = node.minProperties;
    plan.maxProperties = node.maxProperties;
  }

  if (node.type === 'array') {
    if (node.items !== undefined) {
      plan.items = await compileNode(node.items, ctx, depth + 1);
    }
    plan.minItems = node.minItems;
    plan.maxItems = node.maxItems;
    plan.uniqueItems = node.uniqueItems;
  }

  for (const keyword of COMBINATORS) {
    if (Array.isArray(node[keyword])) {
      plan[keyword] = await Promise.all(
        node[keyword].map((branch) => compileNode(branch, ctx, depth + 1)),
      );
    }
  }

  return plan;
}

/**
 * Compile a JSON Schema document into a validation plan.
 *
 * @param {object} schema
 * @param {object} [options]
 * @param {string} [options.tenantId]
 * @param {string} [options.dialect]
 * @param {Function} [options.fetcher]
 * @param {Set<string>} [options.visited]
 * @returns {Promise<object>}
 */
async function compileSchema(schema, options = {}) {
  const ctx = {
    root: schema,
    visited: options.visited || new Set(),
    fetcher: options.fetcher,
    tenantId: options.tenantId !== undefined ? options.tenantId : null,
    dialect: options.dialect || limits.dialectVersion,
  };

  const plan = await compileNode(schema, ctx, 0);
  return {
    ...plan,
    tenantId: ctx.tenantId,
    dialect: ctx.dialect,
    compiledAt: Date.now(),
  };
}

module.exports = {
  compileSchema,
  compilePattern,
  patternCache,
};