# Merging with the baseline

Every route starts from a platform baseline policy and applies the tenant
policy on top. The baseline lives in `src/platform/profile.mjs` and is not
editable by tenants.

## 1. The merge is per directive

### 1.1 Directive-wise

The tenant policy and the baseline policy are merged directive by directive.
A directive that only one side defines is carried over unchanged.

### 1.2 Set semantics

When both sides define the same directive, the merged source list is the union
of the two source lists. Duplicates are removed. The compiler keeps the
baseline sources first and appends the tenant sources that are not already
present, so the output is stable.

For ordinary directives this means a tenant can never remove a source the
baseline requires; it can only add sources. The next section describes the
three directives for which even adding is restricted.

`'none'` denotes the empty set. It contributes nothing to a union: merging
baseline `default-src 'self'` with tenant `default-src 'none'` yields
`default-src 'self'`, and merging `'none'` with `'none'` yields `'none'`. The
result of a merge is never a `'none'` sitting next to another source — that
would be an invalid directive.

## 2. Directives a tenant cannot widen

### 2.1 Locked directives

`object-src`, `frame-ancestors` and `base-uri` are "tenant may tighten only"
directives. For these three:

* A source the tenant adds must already be permitted by the baseline
  directive. Adding a source outside the baseline is a widening attempt: the
  merge is rejected, naming the directive and the source.
* Because of that, the merged value of a locked directive is the tenant's
  source list when the tenant supplies one — a subset of the baseline list —
  and the baseline list otherwise. A locked directive is never a union.
* `'none'` in the tenant list narrows the directive to nothing regardless of
  what the baseline says, because `'none'` as the sole source means "allow
  nothing".

The merged value for a locked directive therefore never contains a source that
was not in the baseline for that directive.

### 2.2 Examples

Baseline `object-src 'none'`, tenant `object-src 'self'`: the tenant is asking
to allow something the baseline forbids, so the merge is rejected.

Baseline `frame-ancestors 'self'`, tenant `frame-ancestors 'none'`: the tenant
tightens, so the merged value is `frame-ancestors 'none'`.

Baseline `frame-ancestors 'self'`, tenant `frame-ancestors https://other.example`:
widening, rejected.

## 3. `report-to`

`report-to` is owned by the platform. The tenant can choose *which* registered
collector receives its reports, but it cannot remove reporting, and it cannot
point the header at an arbitrary value.

* The merged `report-to` directive is always the platform group name
  (`csp-endpoint`). The baseline value wins; a tenant `report-to` never
  replaces it in the policy.
* The tenant's `report-to` value is the collector URL for this route. It is
  validated with `resolve_endpoint()` and, if accepted, becomes the value in
  the separate `Reporting-Endpoints` header. The tenant's raw string is never
  copied into a header.
* When the tenant's value does not resolve to an accepted endpoint, it is
  rejected; it does not silently become a header value.

## 4. What the merge does not do

The merge does not re-validate source expressions that both sides already
carry; that happened when each side was parsed. It does validate the result:
if the union of two valid source lists would produce an invalid directive —
most importantly a `'none'` that is no longer alone — the merge is rejected
rather than emitting an invalid header.
