import { parse_policy } from '../policy/parse.mjs';
import { DEFAULT_REPORT_ENDPOINT } from '../platform/profile.mjs';
import {
  DIRECTIVE_ORDER,
  HEADER_BYTE_LIMIT,
  REPORT_GROUP,
} from '../policy/syntax.mjs';
import { PolicyError } from '../policy/error.mjs';
import { try_resolve_endpoint } from '../report/endpoint.mjs';

// Builds the response headers for a tenant policy.
//
// `input` is a policy text, a policy document, or an already-parsed policy.
// `options.reportEndpoint` is the collector the route is configured with.
//
// Returns { 'Content-Security-Policy': string, 'Reporting-Endpoints': string }.
export function emit_headers(input, options = {}) {
  const input_policy = normalize_input(input);
  const endpoint = reporting_endpoint(input_policy, options);
  const policy = enforce_report_group(input_policy);

  const header_value = serialize(policy);

  return {
    'Content-Security-Policy': header_value,
    'Reporting-Endpoints': `${REPORT_GROUP}="${endpoint}"`,
  };
}

function enforce_report_group(policy) {
  if (!Object.prototype.hasOwnProperty.call(policy, 'report-to')) return policy;
  return { ...policy, 'report-to': [REPORT_GROUP] };
}

export function normalize_input(input) {
  if (input && typeof input === 'object' && !Array.isArray(input)) {
    return input;
  }
  return parse_policy(input);
}

// Serializes a parsed policy into a single header value.
export function serialize(policy) {
  const normalized = normalize_policy_for_serializer(policy);
  const ordered_names = ordered_directive_names(Object.keys(normalized));
  const value = ordered_names
    .map((name) => [name, ...normalized[name].map(sanitize_token)].join(' '))
    .join('; ');

  const byte_length = Buffer.byteLength(value, 'utf8');
  if (byte_length > HEADER_BYTE_LIMIT) {
    throw new PolicyError(
      `Content-Security-Policy is ${byte_length} bytes; limit is ${HEADER_BYTE_LIMIT}`,
      { byteLength: byte_length, limit: HEADER_BYTE_LIMIT },
    );
  }

  return value;
}

function normalize_policy_for_serializer(policy) {
  if (!policy || typeof policy !== 'object' || Array.isArray(policy)) {
    throw new PolicyError('policy must be a directive object', { received: policy });
  }

  const normalized = {};
  for (const [rawName, rawSources] of Object.entries(policy)) {
    const name = sanitize_token(String(rawName)).toLowerCase();
    if (!name) continue;
    if (!Object.prototype.hasOwnProperty.call(normalized, name)) normalized[name] = [];

    const sources = Array.isArray(rawSources) ? rawSources : [rawSources];
    for (const source of sources) {
      if (typeof source !== 'string') {
        throw new PolicyError(`directive "${name}" has a non-string source`, {
          directive: name,
          source,
        });
      }
      normalized[name].push(source);
    }
  }
  return normalized;
}

function ordered_directive_names(names) {
  const known = DIRECTIVE_ORDER.filter((name) => names.includes(name));
  const known_set = new Set(DIRECTIVE_ORDER);
  const unknown = names.filter((name) => !known_set.has(name)).sort();
  return [...known, ...unknown];
}

function sanitize_token(token) {
  return token.replace(/[\u0000-\u001f\u007f;]/g, '');
}

function reporting_endpoint(policy, options) {
  const policy_endpoint = Array.isArray(policy?.['report-to'])
    ? policy['report-to'][0]
    : undefined;
  const candidates = [];

  if (policy_endpoint && policy_endpoint !== REPORT_GROUP) {
    candidates.push(policy_endpoint);
  }
  if (options.reportEndpoint) {
    candidates.push(options.reportEndpoint);
  }

  for (const candidate of candidates) {
    const result = try_resolve_endpoint(candidate, options);
    if (result.ok) return result.value;
    if (options.strictEndpoints || options.strict) throw result.error;
  }

  return DEFAULT_REPORT_ENDPOINT;
}
