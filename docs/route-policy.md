# routeward — route policy

This document defines the behaviour that `routeward` is expected to exhibit.
It is a specification only: it describes observable outcomes, not how they are
produced.

## §1 Terms

- **Request target** — the string in the request line, e.g. `/admin/users?page=2`.
- **Raw path** — the request target with the query string (`?...`) and fragment
  (`#...`) removed.
- **Normalized path** — the result of applying §2 to the raw path.
- **Effective method** — see §4.
- **Rule** — one entry of the `rules` array in `config/routes.json`, identified
  by its `id`, carrying a `pattern`, a `methods` list, a `scopes` list, and an
  optional `tenants` list.
- **Session** — the record produced by authentication: `user`, `tenant`, a set
  of `scopes`, and an opaque `id`.

## §2 Path normalization

Rule matching is always performed against the normalized path. The raw path is
never used for authorization decisions. Normalization proceeds in this order:

1. **Reject encoded separators.** If the raw path contains a percent-encoded
   forward slash (`%2f`, `%2F`) or backslash (`%5c`, `%5C`), in any letter case,
   the request is rejected with `400`. This check happens before any decoding
   and must not be defeated by additional encoding.
2. **Decode exactly once.** Percent-escapes are decoded a single time. A
   sequence such as `%252e` decodes to `%2e`; the result is *not* decoded a
   second time. A malformed escape (for example `%zz` or a trailing `%`) is
   rejected with `400`.
3. **Reject residual escapes that hide structure.** After the single decode,
   if the path still contains an escape for a dot, a slash, a backslash, or a
   percent sign (`%2e`, `%2f`, `%5c`, `%25`, in any letter case), the request is
   rejected with `400`. This is what stops double-encoded traversal such as
   `%252e%252e` (which decodes to `%2e%2e`) from being carried into matching.
4. **Collapse and resolve.** Runs of `/` collapse to a single `/`. Segments
   consisting of `.` are removed. Segments consisting of `..` remove the
   preceding segment; `..` at the root is discarded. The result always begins
   with `/`, never ends with `/` (except when it is exactly `/`), and contains
   no `.` or `..` segments.
5. **Rejected input** — any request failing step 1, 2, or 3 is answered with
   `400` and never reaches authorization or a handler.

Normalization is idempotent: normalizing a normalized path yields the same
path.

### §2.1 Examples

| Raw path | Normalized path |
|---|---|
| `/admin//users` | `/admin/users` |
| `/admin/./users` | `/admin/users` |
| `/admin/%2e%2e/public` | `/public` |
| `/admin/%2e%2e%2fpublic` | rejected (`400`) |
| `/admin%2Fusers` | rejected (`400`) |
| `/admin/%252e%252e/public` | rejected (`400`) |
| `/a/b/../../c` | `/c` |
| `/../../etc` | `/etc` |
| `/admin/users/` | `/admin/users` |

## §3 Rule matching

A rule matches a request when all of the following hold:

1. The rule's `pattern` matches the **normalized path** (§2).
2. The effective method (§4) appears in the rule's `methods` list.
3. Either the rule has no `tenants` list, or the session's `tenant` is an
   element of that list.

### §3.1 Pattern syntax

- A pattern must be an absolute path beginning with `/`.
- `*` is a **single-segment wildcard**. It matches one or more characters
  within one path segment and never matches `/`.
- Every other character is literal. `.`, `+`, `?`, `(`, `)`, `[`, `]`, `{`,
  `}`, `|`, `^`, `$` and `\` carry no special meaning inside a pattern.
- Matching is anchored: a pattern must consume the entire normalized path.
- Example: `/api/*/secret` matches `/api/orders/secret` but not
  `/api/a/b/secret`.

### §3.2 Selecting among matches

When several rules match, exactly one is selected:

1. **Longest literal prefix wins.** The literal prefix of a pattern is the
   portion before its first `*` (or the whole pattern when it has none). The
   rule whose literal prefix is longest is selected.
2. **Tie-break on specificity.** If two candidates have equal literal prefix
   length, the one with more non-wildcard segments wins.
3. **Deterministic remainder.** If a tie remains, the rule appearing earlier in
   the configuration file wins. This is a stability guarantee only; it must not
   be observable for any pair of distinct patterns that both match a path.

Declaration order in the file is never itself a reason to select a rule: a
narrower rule always beats a broader one that also matches, regardless of where
either appears in the file.

### §3.3 No match

If no rule matches, the request is **denied** with `403`. It never reaches a
handler. There is no implicit "allow" path and no default rule.

## §4 Effective method

The method used for authorization is the method declared on the request line,
unless the request carries `X-HTTP-Method-Override`, in which case the header
value (trimmed, upper-cased) is the effective method.

Authorization must be evaluated against the effective method. An override never
grants access: the effective method still has to appear in the selected rule's
`methods`. Consequently `GET /admin/users` with `X-HTTP-Method-Override: DELETE`
is authorized exactly like `DELETE /admin/users`.

The override header only changes which rule applies; it does not change the
handler contract beyond the effective method.

## §5 Authorization result

For the selected rule:

- Every scope in the rule's `scopes` list must be present in the session's
  scopes. A rule with an empty `scopes` list requires no scope.
- If any required scope is missing, the response is `403` and the handler is
  not invoked. The response body identifies the rule and the missing scopes.
- If all required scopes are present, the request proceeds to the handler.

A rule that matches but is not selected (§3.2) does not constrain the request:
only the selected rule's `scopes` and `methods` apply.

## §6 Tenant isolation

`tenants` is an allow-list of tenant identifiers. A rule with a `tenants` list
applies only to sessions whose `tenant` matches exactly. A session must not be
able to reach a rule scoped to a different tenant, and a tenant-scoped rule must
not fall back to a broader tenant-neutral rule unless that broader rule
independently matches.

The `tenant` value comes from the authenticated session. Any tenant identifier
supplied by the client (header, query parameter, cookie other than the session
cookie) is ignored.

## §7 Response contract

| Condition | Status |
|---|---|
| Missing or unknown session cookie | `401` |
| Malformed or rejected path (§2, steps 1–3) | `400` |
| No matching rule (§3.3) | `403` |
| Matching rule, missing scope (§5) | `403` |
| Authorized | handler's own status |
| Authorized, but no handler for the effective method and normalized path | `404` |

Authorization decisions are made on the same path and method values that
handlers observe: the normalized path and the effective method.
