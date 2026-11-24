import { BASELINE_POLICY, LOCKED_DIRECTIVES } from '../platform/profile.mjs';
import { REPORT_GROUP } from '../policy/syntax.mjs';
import { PolicyError } from '../policy/error.mjs';
import { resolve_endpoint } from '../report/endpoint.mjs';
import { normalize_source } from './parse.mjs';

// Merges a tenant policy with the platform baseline.
//
// `tenant` is a parsed policy ({ name: [source, ...] }).  `baseline` defaults
// to the platform baseline.  Returns a new policy object; neither input is
// mutated.
export function merge_with_baseline(tenant, baseline = BASELINE_POLICY, options = {}) {
  const tenantPolicy = canonicalize_policy(tenant || {});
  const basePolicy = canonicalize_policy(baseline || {});
  const merged = {};

  for (const [name, sources] of Object.entries(basePolicy)) {
    merged[name] = unique_sources(sources);
  }

  const tenantEndpoint = tenantPolicy['report-to']?.[0];
  if (
    tenantEndpoint !== undefined &&
    tenantEndpoint !== REPORT_GROUP &&
    options.validateEndpoint !== false
  ) {
    resolve_endpoint(tenantEndpoint, options);
  }

  for (const [name, sources] of Object.entries(tenantPolicy)) {
    if (name === 'report-to') {
      merged[name] = Array.isArray(basePolicy[name]) && basePolicy[name].length > 0
        ? unique_sources(basePolicy[name])
        : [REPORT_GROUP];
      continue;
    }

    if (LOCKED_DIRECTIVES.includes(name)) {
      merged[name] = merge_locked(name, sources, basePolicy[name] || []);
    } else {
      merged[name] = merge_union(merged[name] || [], sources);
    }

    validate_none_result(name, merged[name]);
  }

  if (Object.prototype.hasOwnProperty.call(merged, 'report-to')) {
    merged['report-to'] = [REPORT_GROUP];
  }

  return merged;
}

function canonicalize_policy(policy) {
  const result = {};
  for (const [rawName, rawSources] of Object.entries(policy)) {
    const name = rawName.toLowerCase();
    if (!result[name]) result[name] = [];
    if (!Array.isArray(rawSources)) {
      result[name].push(normalize_source(rawSources));
      continue;
    }
    for (const source of rawSources) {
      const normalizedSource = normalize_source(source);
      if (!result[name].includes(normalizedSource)) result[name].push(normalizedSource);
    }
  }
  return result;
}

function unique_sources(sources) {
  return [...new Set(sources)];
}

function merge_union(baselineSources, tenantSources) {
  const baseline = unique_sources(baselineSources);
  const tenant = unique_sources(tenantSources);
  const hasNone = baseline.includes("'none'") || tenant.includes("'none'");
  const result = [];

  for (const source of [...baseline, ...tenant]) {
    if (source === "'none'") continue;
    if (!result.includes(source)) result.push(source);
  }

  if (result.length > 0) return result;
  return hasNone ? ["'none'"] : [];
}

function merge_locked(name, tenantSources, baselineSources) {
  const tenant = unique_sources(tenantSources);
  if (tenant.includes("'none'")) {
    validate_none_result(name, tenant);
    return ["'none'"];
  }

  const allowed = new Set(unique_sources(baselineSources));
  for (const source of tenant) {
    if (!allowed.has(source)) {
      throw new PolicyError(
        `directive "${name}" cannot be widened with source ${JSON.stringify(source)}`,
        { directive: name, source },
      );
    }
  }

  return tenant;
}

function validate_none_result(name, sources) {
  if (sources.includes("'none'") && sources.length > 1) {
    throw new PolicyError(
      `merged directive "${name}" cannot combine 'none' with another source`,
      { directive: name, source: "'none'", sources: sources.slice() },
    );
  }
}
