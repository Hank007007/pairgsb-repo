import { REPORT_GROUP } from '../policy/syntax.mjs';

// Policy applied to every route before the tenant policy is taken into
// account.  See docs/merge.md.
export const BASELINE_POLICY = Object.freeze({
  'default-src': Object.freeze(["'self'"]),
  'script-src': Object.freeze(["'self'"]),
  'style-src': Object.freeze(["'self'"]),
  'img-src': Object.freeze(["'self'", 'https://cdn.platform.example']),
  'connect-src': Object.freeze(["'self'"]),
  'object-src': Object.freeze(["'none'"]),
  'base-uri': Object.freeze(["'self'"]),
  'frame-ancestors': Object.freeze(["'self'"]),
  'report-to': Object.freeze([REPORT_GROUP]),
});

// Collector used when a route has no tenant endpoint of its own.
export const DEFAULT_REPORT_ENDPOINT =
  'https://reports.platform.example/csp/collect';

// Directives a tenant may tighten but not widen; see docs/merge.md §2.1.
export const LOCKED_DIRECTIVES = Object.freeze([
  'object-src',
  'frame-ancestors',
  'base-uri',
]);
