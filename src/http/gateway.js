'use strict';

const limits = require('../policy/limits');
const { compileSchema } = require('../schema/compile');
const { coerceInstance, applyDefaults } = require('../validate/coerce');
const { walk } = require('../validate/walk');
const { SchemaError } = require('../schema/errors');

/**
 * Compiled plans, shared for the lifetime of the process.
 * @type {Map<string, object>}
 */
const planCache = new Map();

/**
 * Read a request body into a UTF-8 string.
 *
 * Accepts a string, a Buffer, an async iterable, an object exposing
 * `read()` (returning a chunk or `null` / a promise thereof), or an
 * already-parsed instance.
 *
 * @param {unknown} body
 * @returns {Promise<unknown>}
 */
async function readBody(body) {
  if (typeof body === 'string') return body;
  if (Buffer.isBuffer(body)) return body.toString('utf8');

  if (body && typeof body === 'object' && !Array.isArray(body)) {
    if (typeof body[Symbol.asyncIterator] === 'function') {
      const chunks = [];
      for await (const chunk of body) {
        chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(String(chunk)));
      }
      return Buffer.concat(chunks).toString('utf8');
    }
    if (typeof body.read === 'function') {
      const chunks = [];
      for (;;) {
        const chunk = await body.read();
        if (chunk === null || chunk === undefined) break;
        chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(String(chunk)));
      }
      return Buffer.concat(chunks).toString('utf8');
    }
  }

  // Already-parsed JSON instance.
  return body;
}

/**
 * Get (or build) the compiled plan for a schema.
 *
 * @param {object} schema
 * @param {object} options
 * @returns {Promise<object>}
 */
async function getPlan(schema, options) {
  const key = JSON.stringify(schema);
  const cached = planCache.get(key);
  if (cached) return cached;

  const plan = await compileSchema(schema, {
    tenantId: options.tenantId,
    dialect: options.dialect,
    fetcher: options.fetcher,
  });
  planCache.set(key, plan);
  return plan;
}

/**
 * Validate an incoming request body against a tenant schema.
 *
 * @param {object} schema tenant-supplied JSON Schema
 * @param {unknown} body raw body (string / buffer / reader) or parsed instance
 * @param {object} [options]
 * @param {string} [options.tenantId]
 * @param {string} [options.dialect]
 * @param {Function} [options.fetcher]
 * @returns {Promise<{ok: boolean, instance?: unknown, errors?: object[]}>}
 */
async function validateRequest(schema, body, options = {}) {
  const raw = await readBody(body);

  let instance;
  if (typeof raw === 'string') {
    try {
      instance = JSON.parse(raw);
    } catch (cause) {
      return {
        ok: false,
        errors: [{ path: '', keyword: 'parse', message: 'request body is not valid JSON' }],
      };
    }
    if (Buffer.byteLength(raw, 'utf8') > limits.maxBodyBytes) {
      return {
        ok: false,
        errors: [{ path: '', keyword: 'body', message: 'request body exceeds limit' }],
      };
    }
  } else {
    instance = raw;
  }

  let plan;
  try {
    plan = await getPlan(schema, options);
  } catch (e) {
    return {
      ok: false,
      errors: [
        {
          path: '',
          keyword: 'schema',
          message: e && e.message ? e.message : 'schema compilation failed',
          ref: e && e.ref ? e.ref : undefined,
          stack: e && e.stack ? e.stack : undefined,
        },
      ],
    };
  }

  coerceInstance(plan, instance);
  applyDefaults(plan, instance);

  const errors = [];
  walk(plan, instance, [], errors, 0);

  if (errors.length > 0) {
    return {
      ok: false,
      errors,
      instance,
      ref: plan.ref,
    };
  }

  return { ok: true, instance };
}

module.exports = {
  validateRequest,
  getPlan,
  planCache,
  readBody,
};