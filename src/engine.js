import { decideMany } from './batch.js';
import { TtlCache } from './cache.js';
import { evaluate } from './conditions.js';
import { compileGlob } from './glob.js';
import { isPlainObject, normalizeRules } from './rules.js';

function normalizeResource(resource) {
  let path = resource.trim();
  if (!path.startsWith('/')) {
    path = `/${path}`;
  }
  if (path.length > 1) {
    path = path.replace(/\/+$/, '');
  }
  return path.length === 0 ? '/' : path;
}

function readRequest(request) {
  if (!isPlainObject(request)) {
    throw new TypeError('request must be an object');
  }

  const { principal, resource, action } = request;
  if (typeof principal !== 'string' || principal === '') {
    throw new TypeError('request.principal must be a non-empty string');
  }
  if (typeof resource !== 'string' || resource === '') {
    throw new TypeError('request.resource must be a non-empty string');
  }
  if (typeof action !== 'string' || action === '') {
    throw new TypeError('request.action must be a non-empty string');
  }

  let context = request.context;
  if (context === undefined || context === null) {
    context = {};
  } else if (!isPlainObject(context)) {
    throw new TypeError('request.context must be an object');
  }

  return { principal, resource, action, context };
}

/**
 * An embeddable policy decision point.
 */
export class PolicyEngine {
  #rules;
  #compiled;
  #cache;

  /**
   * @param {{ rules?: Array<object>, cache?: TtlCache | false | null, ttlMs?: number }} [options]
   */
  constructor({ rules = [], cache, ttlMs = 60_000 } = {}) {
    this.#rules = normalizeRules(rules, { source: 'engine' });
    this.#compiled = this.#rules.map((rule) => ({
      rule,
      principal: compileGlob(rule.principal),
      resource: compileGlob(rule.resource),
    }));

    if (cache === false || cache === null) {
      this.#cache = null;
    } else if (cache instanceof TtlCache) {
      this.#cache = cache;
    } else {
      this.#cache = new TtlCache({ ttlMs });
    }
  }

  get rules() {
    return this.#rules;
  }

  get cache() {
    return this.#cache;
  }

  /**
   * Resource paths that policy rules may bind to for a request.
   *
   * @param {string} resource
   * @returns {string[]}
   */
  candidates(resource) {
    return [normalizeResource(resource)];
  }

  #matchesResource(entry, resource) {
    for (const candidate of this.candidates(resource)) {
      if (entry.resource.test(candidate)) {
        return true;
      }
    }
    return false;
  }

  /**
   * Decide a single authorization request.
   *
   * @param {{ principal: string, resource: string, action: string, context?: object }} request
   * @returns {{ allow: boolean, matchedPolicy: string | null, reason: string }}
   */
  decide(request) {
    const { principal, resource, action, context } = readRequest(request);

    for (const entry of this.#compiled) {
      const { rule } = entry;

      if (!entry.principal.test(principal)) {
        continue;
      }
      if (!this.#matchesResource(entry, resource)) {
        continue;
      }
      if (!rule.action.includes(action)) {
        continue;
      }
      if (!evaluate(rule.condition, context)) {
        continue;
      }

      if (rule.effect === 'deny') {
        return {
          allow: false,
          matchedPolicy: rule.id,
          reason: `matched deny rule ${rule.id}`,
        };
      }

      return {
        allow: true,
        matchedPolicy: rule.id,
        reason: `matched permit rule ${rule.id}`,
      };
    }

    return {
      allow: true,
      matchedPolicy: null,
      reason: 'no matching policy',
    };
  }

  /**
   * Decide a batch of authorization requests.
   *
   * @param {Array<object>} requests
   * @returns {Array<{ allow: boolean, matchedPolicy: string | null, reason: string }>}
   */
  decideMany(requests) {
    return decideMany(this, requests, this.#cache);
  }
}
