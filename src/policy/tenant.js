'use strict';

class PolicyError extends Error {
  constructor(message) {
    super(message);
    this.name = 'PolicyError';
  }
}

/**
 * A tenant policy has the following shape:
 *
 *   {
 *     tenantId: string,        // tenant this policy applies to
 *     root: string,            // absolute path prefix owned by the tenant
 *     adminActions: string[],  // actions the tenant administrator holds
 *     defaultTtlMs: number,    // lifetime used when a request omits ttlMs
 *     maxTtlMs: number,        // upper bound on any single grant lifetime
 *   }
 */

/**
 * Build a lookup table of tenant policies keyed by tenant id.
 *
 * @param {Array<object>} policies
 * @returns {{ get(tenantId: string): object | undefined, list(): object[] }}
 */
function createPolicyRegistry(policies = []) {
  const byTenant = new Map();
  for (const policy of policies) {
    if (policy === null || typeof policy !== 'object') {
      throw new PolicyError('policy entry must be an object');
    }
    if (typeof policy.tenantId !== 'string' || policy.tenantId.length === 0) {
      throw new PolicyError('policy entry is missing tenantId');
    }
    byTenant.set(policy.tenantId, { ...policy });
  }

  return {
    get(tenantId) {
      return byTenant.get(tenantId);
    },
    list() {
      return Array.from(byTenant.values());
    },
  };
}

/**
 * Read the policy that applies to a tenant.
 *
 * @param {{ get(tenantId: string): object | undefined }} registry
 * @param {string} tenantId
 * @returns {object}
 */
function loadPolicy(registry, tenantId) {
  const policy = registry.get(tenantId);
  if (!policy) {
    return {};
  }
  return policy;
}

module.exports = { PolicyError, createPolicyRegistry, loadPolicy };
