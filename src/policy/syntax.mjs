// Shared vocabulary for the CSP policy compiler.

export const POLICY_KEYWORDS = Object.freeze([
  "'none'",
  "'self'",
  "'unsafe-inline'",
  "'unsafe-eval'",
  "'strict-dynamic'",
  "'unsafe-hashes'",
  "'wasm-unsafe-eval'",
  "'report-sample'",
  "'inline-speculation-rules'",
]);

// The same keywords without their surrounding quotes.
export const POLICY_KEYWORD_INNER_SET = new Set(
  POLICY_KEYWORDS.map((keyword) => keyword.slice(1, -1)),
);

export const HASH_ALGORITHMS = Object.freeze(['sha256', 'sha384', 'sha512']);

// Directives are emitted in this order; see docs/serialize.md §2.
export const DIRECTIVE_ORDER = Object.freeze([
  'default-src',
  'script-src',
  'script-src-elem',
  'script-src-attr',
  'style-src',
  'style-src-elem',
  'style-src-attr',
  'img-src',
  'font-src',
  'connect-src',
  'media-src',
  'object-src',
  'frame-src',
  'child-src',
  'worker-src',
  'manifest-src',
  'prefetch-src',
  'base-uri',
  'form-action',
  'frame-ancestors',
  'sandbox',
  'upgrade-insecure-requests',
  'block-all-mixed-content',
  'require-trusted-types-for',
  'trusted-types',
  'report-uri',
  'report-to',
]);

// Upper bound for a single header value; see docs/serialize.md §3.
export const HEADER_BYTE_LIMIT = 4096;

// Reporting group advertised by every route.
export const REPORT_GROUP = 'csp-endpoint';
