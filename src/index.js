import { PolicyEngine } from './engine.js';
import { loadPolicies } from './policy-store.js';

export { decideMany } from './batch.js';
export { TtlCache } from './cache.js';
export { evaluate } from './conditions.js';
export { PolicyEngine } from './engine.js';
export { compileGlob, matchGlob } from './glob.js';
export { loadPolicies, parsePolicyDocument } from './policy-store.js';
export { PolicyFormatError, normalizeRule, normalizeRules } from './rules.js';

/**
 * Create an engine from an explicit rule list or from a policy directory.
 *
 * @param {{ policyDir?: string, rules?: Array<object>, cache?: import('./cache.js').TtlCache | false | null, ttlMs?: number }} [options]
 * @returns {Promise<PolicyEngine>}
 */
export async function createEngine({ policyDir = './policies', rules, cache, ttlMs } = {}) {
  const resolvedRules = rules ?? (await loadPolicies(policyDir));
  return new PolicyEngine({ rules: resolvedRules, cache, ttlMs });
}
