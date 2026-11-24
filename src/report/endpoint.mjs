import { DEFAULT_REPORT_ENDPOINT } from '../platform/profile.mjs';
import { PLATFORM_DOMAINS, TENANT_REGISTRY } from '../platform/registry.mjs';
import { REPORT_GROUP } from '../policy/syntax.mjs';

// Resolves a tenant-supplied reporting endpoint.
//
// `options.tenant` names the tenant the endpoint was supplied by; the
// registry entry it selects is available through `registered_domains()`.
// Returns the endpoint to write into `Reporting-Endpoints`.
export function resolve_endpoint(endpoint, options = {}) {
  let url;
  try {
    url = new URL(endpoint);
  } catch {
    return endpoint;
  }

  return url.href || endpoint;
}

export function registered_domains(tenant, registry = TENANT_REGISTRY) {
  const own = registry[tenant] || [];
  return [...PLATFORM_DOMAINS, ...own];
}

export { DEFAULT_REPORT_ENDPOINT, REPORT_GROUP };
