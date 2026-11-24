import { BASELINE_POLICY } from '../platform/profile.mjs';

// Merges a tenant policy with the platform baseline.
//
// `tenant` is a parsed policy ({ name: [source, ...] }).  `baseline` defaults
// to the platform baseline.  Returns a new policy object; neither input is
// mutated.
export function merge_with_baseline(tenant, baseline = BASELINE_POLICY) {
  const tenantPolicy = tenant || {};
  const basePolicy = baseline || {};
  const merged = {};

  for (const [name, sources] of Object.entries(basePolicy)) {
    merged[name] = union([], sources);
  }

  for (const [name, sources] of Object.entries(tenantPolicy)) {
    if (name === 'report-to') {
      merged[name] = sources.slice();
      continue;
    }
    merged[name] = union(merged[name] || [], sources);
  }

  return merged;
}

function union(existing, incoming) {
  const set = new Set(existing);
  for (const source of incoming) set.add(source);
  return [...set];
}
