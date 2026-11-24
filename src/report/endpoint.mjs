import { DEFAULT_REPORT_ENDPOINT } from '../platform/profile.mjs';
import { PLATFORM_DOMAINS, TENANT_REGISTRY } from '../platform/registry.mjs';
import { REPORT_GROUP } from '../policy/syntax.mjs';
import { PolicyError } from '../policy/error.mjs';

// Resolves a tenant-supplied reporting endpoint.
//
// `options.tenant` names the tenant the endpoint was supplied by; the
// registry entry it selects is available through `registered_domains()`.
// Returns the endpoint to write into `Reporting-Endpoints`.
export function resolve_endpoint(endpoint, options = {}) {
  const resolvedOptions = typeof options === 'string' ? { tenant: options } : (options || {});
  if (typeof endpoint !== 'string' || endpoint === '') {
    throw endpoint_error(endpoint, 'endpoint must be an absolute https URL');
  }
  if (/[\u0000-\u001f\u007f]/.test(endpoint)) {
    throw endpoint_error(endpoint, 'endpoint contains a control character');
  }

  let url;
  try {
    url = new URL(endpoint);
  } catch {
    throw endpoint_error(endpoint, 'endpoint must be an absolute URL');
  }

  if (url.protocol !== 'https:') {
    throw endpoint_error(endpoint, `endpoint scheme "${url.protocol}" is not allowed`);
  }

  const tenant = resolvedOptions.tenant ?? resolvedOptions.tenantId;
  const registry = resolvedOptions.registry ?? TENANT_REGISTRY;
  const domains = registered_domains(tenant, registry);
  const host = url.hostname;
  const registered = domains.some(
    (domain) => host === domain || host.endsWith(`.${domain}`),
  );
  if (!registered) {
    throw endpoint_error(endpoint, `endpoint host "${host}" is not registered`);
  }

  if (!url.pathname.startsWith('/csp/')) {
    throw endpoint_error(endpoint, 'endpoint path must start with "/csp/"');
  }

  return url.href || endpoint;
}

function endpoint_error(endpoint, reason) {
  return new PolicyError(`invalid reporting endpoint ${JSON.stringify(endpoint)}: ${reason}`, {
    endpoint,
    reason,
  });
}

export function try_resolve_endpoint(endpoint, options = {}) {
  try {
    return { ok: true, value: resolve_endpoint(endpoint, options) };
  } catch (error) {
    return { ok: false, error };
  }
}

export function registered_domains(tenant, registry = TENANT_REGISTRY) {
  const own = registry[tenant] || [];
  return [...PLATFORM_DOMAINS, ...own];
}

export { DEFAULT_REPORT_ENDPOINT, REPORT_GROUP };
