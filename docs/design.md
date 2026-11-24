# Design

cspfence turns a tenant's CSP policy into the two headers the CDN edge writes.
This document describes the intended behaviour of each stage and the contract
between them. The four documents under `docs/` give the normative details.

## 1. What the edge needs from this layer

For every response the edge writes exactly two security headers:

* `Content-Security-Policy` — the tenant policy merged with the platform
  baseline, serialized once.
* `Reporting-Endpoints` — the mapping from the reporting group to a collector
  URL the platform has vetted.

Both values are written verbatim by the edge, so this layer is the only place
where a tenant string is turned into a header. Two properties follow:

* A tenant does not widen protection beyond the baseline. It may narrow it.
* No tenant-supplied character reaches the header unexamined. Anything that
  could end a header value — CR, LF, a stray `;`, an oversized value — is
  rejected before the header is built.

## 2. The stages

### 2.1 Lexing — `tokenize()`

Turns policy text into tokens. A token records its text and whether it is the
last token of a directive. Separators and directive delimiters follow
`docs/policy-grammar.md` §2.1. Lexing performs no validation.

### 2.2 Parsing — `parse_policy()`

Assembles tokens, or a JSON document, into a directive table keyed by the
canonical directive name. This stage covers:

* directive-name normalization, so `Script-Src` and `script-src` are one key
  (`docs/policy-grammar.md` §2.2);
* repeated directives, which combine into one directive by union of their
  sources (§2.2);
* source-expression normalization where the grammar defines an equivalence
  (quoted keywords are case-insensitive, §2.2);
* validation of every source expression through
  `validate_source_expression()`, with the error naming the directive and the
  expression (§3, §4);
* the `'none'` rule (§2.3).

The baseline is not visible at this stage.

### 2.3 Merging — `merge_with_baseline()`

Combines a parsed tenant policy with the baseline (`docs/merge.md`):

* set semantics per directive — union for ordinary directives, with `'none'`
  acting as the empty set (§1.2);
* the tighten-only rule for `object-src`, `frame-ancestors` and `base-uri`
  (§2.1);
* platform ownership of `report-to` (§3);
* rejection of a merged directive that would be illegal.

### 2.4 Serialization — `emit_headers()`

Builds the final header strings (`docs/serialize.md`):

* the fixed directive order with `default-src` first (§2);
* the single-header format (§1);
* the 4096-byte ceiling, enforced by refusing to emit rather than truncating
  (§3);
* removal of control characters from the value (§3);
* the resolved endpoint in `Reporting-Endpoints`, as produced by
  `resolve_endpoint()` (`docs/reports.md`).

This is the last stage before the header is written.

### 2.5 Endpoint resolution — `resolve_endpoint()`

Decides whether a tenant-supplied URL may appear in a header: scheme,
registered domain, `/csp/` path prefix, and absolute form (`docs/reports.md`
§1). Its return value is what `emit_headers()` places in `Reporting-Endpoints`.

## 3. Data flow between the stages

1. text becomes tokens in `tokenize()`;
2. tokens become a directive table in `parse_policy()`;
3. the tenant table and the baseline become a merged table in
   `merge_with_baseline()`;
4. the merged table becomes the `Content-Security-Policy` header value in
   `emit_headers()`;
5. the tenant endpoint becomes an accepted endpoint in `resolve_endpoint()`,
   and that accepted endpoint becomes the `Reporting-Endpoints` value.

Each stage consumes the output of the stage before it. The header value is
built from the merged table, so every header reflects both the tenant policy
and the baseline.

## 4. Failure behaviour

Every stage rejects with a `PolicyError` that names the directive and, where
relevant, the source expression or endpoint. A policy that cannot be compiled
is reported to the operator; it does not become a header.

## 5. Out of scope

* Rendering the console UI or validating the JSON schema at the API edge.
* Route matching: which routes get a policy is decided outside this module.
* Nonce generation and hash computation: this layer accepts or rejects a
  nonce or hash, it does not produce one.
