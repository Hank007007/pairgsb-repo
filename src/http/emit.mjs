import { parse_policy } from '../policy/parse.mjs';
import { DEFAULT_REPORT_ENDPOINT } from '../platform/profile.mjs';
import { REPORT_GROUP } from '../policy/syntax.mjs';

// Builds the response headers for a tenant policy.
//
// `input` is a policy text, a policy document, or an already-parsed policy.
// `options.reportEndpoint` is the collector the route is configured with.
//
// Returns { 'Content-Security-Policy': string, 'Reporting-Endpoints': string }.
export function emit_headers(input, options = {}) {
  const policy = normalize_input(input);

  const header_value = serialize(policy);

  const tenant_endpoint = policy['report-to'] && policy['report-to'].length > 0
    ? policy['report-to'][0]
    : '';
  const endpoint = tenant_endpoint || options.reportEndpoint || DEFAULT_REPORT_ENDPOINT;

  return {
    'Content-Security-Policy': header_value,
    'Reporting-Endpoints': `${REPORT_GROUP}="${endpoint}"`,
  };
}

export function normalize_input(input) {
  if (input && typeof input === 'object' && !Array.isArray(input)) {
    return input;
  }
  return parse_policy(input);
}

// Serializes a parsed policy into a single header value.
export function serialize(policy) {
  return Object.entries(policy)
    .map(([name, sources]) => [name, ...sources].join(' '))
    .join('; ');
}
