# Query admission policy

This document is the normative description of what `gqladmit` must accept and
what it must reject. It describes observable behaviour only; how the
implementation achieves it is out of scope.

## 1. Scope

Every request that reaches the gateway passes through the admission layer
before it is forwarded. One shared backend schema serves all tenants, so the
policy has to hold for the worst query any tenant can send, not only for the
queries a well-behaved client happens to send.

A request is either **admitted** (the gateway forwards it unchanged) or
**rejected** (the gateway returns a 400-class error and forwards nothing).
Rejection must never depend on the backend being able to survive the query.

## 2. Definitions

**Fragment expansion.** A query is evaluated as if every named fragment spread
(`...Name`) were replaced, in place, by the body of that fragment, recursively,
and every inline fragment (`... on Type { ... }`) were replaced by its
selection set. Counting and policy checks apply to the expanded form.

**Response key.** The key under which a field appears in the response: the
alias when one is written, otherwise the field name.

**Real field name.** The schema field a selection refers to: the identifier
after the colon when an alias is written, otherwise the identifier itself.

**Depth.** The number of field nodes on the longest path from the operation
root down to a leaf field. A top-level field has depth 1; a top-level field
with one nested field has depth 2. Inline fragments do not add a level; the
fields they contain sit at the depth of their enclosing position.

**Field count.** The number of field nodes in the expanded form, counting each
alias separately. Selecting `a: title b: title` counts two fields.

## 3. Structural rules

### 3.1 Fragment cycles

A fragment graph containing a cycle — including a fragment that refers to
itself, directly or through other fragments — is invalid and the request is
rejected. Cycle handling must be part of normal rejection: a malformed request
may not terminate the process, exhaust the stack, or hang the admission layer.

### 3.2 Repeated fragments

A fragment may be referenced from more than one place. Each reference
contributes its own copy of the fragment body to the expanded form, and
therefore it contributes to the field count and the depth for every reference.
Referencing the same fragment twice does not make the second reference
disappear.

### 3.3 Unknown fragments

A spread that names a fragment not defined in the document is rejected.

## 4. Cost limits

| Limit | Value |
|---|---|
| Maximum depth | 8 |
| Maximum fields | 200 |

Both limits are checked against the expanded form described in §2.

- A query whose expanded depth exceeds 8 is rejected.
- A query whose expanded field count exceeds 200 is rejected.
- Field count is per response key: aliases count individually, so one field
  selected under three aliases counts three times.
- Directives do not change the count. Fields carrying `@skip(if: true)` or
  omitted by `@include(if: false)` are still counted, because the admission
  decision is made before variables are known.
- Fragment bodies count wherever they are spread. A fragment that hides a deep
  or wide selection does not reduce the measured depth or field count.

## 5. Introspection

The public gateway exposes the meta fields `__typename`, `__schema` and
`__type` in the schema namespace. Only `__typename` is available to ordinary
callers.

- Any field in the meta namespace other than `__typename` is rejected. In
  practice the gateway rejects `__schema` and `__type`, plus any other `__`
  prefixed field that is not `__typename`.
- The decision is made on the **real field name**, not the response key. An
  alias does not change which schema field is selected.
- Wrapping the field in a named fragment or an inline fragment does not change
  the decision: after expansion the field is checked like any other.
- A rejection identifies the offending field name so that the caller can see
  why the request was refused.

## 6. Field authorisation

Each tenant has a set of fields it may read. Fields not listed as restricted
are readable by every tenant; restricted fields are readable only by tenants
that were explicitly granted them.

- The decision is made on the **real field name**. An alias is a response
  formatting device; it does not rename the schema field for authorisation
  purposes. Selecting `me: userSecret` asks for `userSecret` and must be
  judged as `userSecret`.
- The check applies after fragment expansion, so a restricted field reached
  through a named fragment or an inline fragment is judged the same way as one
  written directly.
- A rejection identifies the tenant and the field name.

## 7. Error reporting

Rejections carry a stable code so that gateways, dashboards and tests can
distinguish the reasons:

| Code | Meaning |
|---|---|
| `SYNTAX` | The document could not be parsed, does not contain exactly one operation, references an unknown fragment, or contains a fragment cycle. |
| `INTROSPECTION` | A meta field other than `__typename` was selected. |
| `FORBIDDEN` | A restricted field was selected by a tenant without a grant. |
| `DEPTH` | The expanded depth exceeds the configured maximum. |
| `FIELDS` | The expanded field count exceeds the configured maximum. |

The order of checks is: syntax and structure, introspection, authorisation,
depth, field count. A request that fails several checks reports the first one
in that order.
