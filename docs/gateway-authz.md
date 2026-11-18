# hopguard — Gateway Authorization Specification

Status: **normative**. This document defines the behaviour the gateway must
exhibit. Where the running service and this document disagree, this document
wins.

Terms used below:

- **caller** — the HTTP client talking to the gateway.
- **upstream** — the internal service that owns a route prefix.
- **session token** — the compact RS256 JWT presented in
  `Authorization: Bearer <token>`.
- **identity** — the `sub` and `roles` carried by a verified session token.

---

## §1 Request pipeline

Every request is processed in this order. A failure at any step terminates the
request; later steps do not run.

1. Resolve the route from the request target (`§2`).
2. If the route is not public, verify the session token (`§4`).
3. If the route declares scopes, evaluate them against the token (`§3`).
4. Build the upstream request, stripping caller-supplied identity headers and
   writing the verified identity (`§5`).
5. Forward to the route's upstream and relay the response.

No response may be forwarded from an upstream for a request that failed step 2
or step 3. Failures are reported using the codes in `§6`.

---

## §2 Path normalization and route resolution

### §2.1 Normalization

Route resolution operates on a **normalized path**, never on the raw request
target.

1. The path component of the request target is taken; the query string and
   fragment are excluded from matching.
2. Percent-encoded octets are decoded exactly **once**. A sequence that is
   still percent-encoded after one decoding pass is not decoded again.
3. The decoded path is normalized by removing `.` segments and resolving `..`
   segments against their parent, as defined by RFC 3986 §5.2.4
   (`remove_dot_segments`). A path that walks above the root is clamped at the
   root: `/../../x` normalizes to `/x`. Consecutive slashes collapse to one.
4. A request target whose path cannot be decoded (malformed percent escape) or
   that is not an absolute path is rejected with `400 bad_request` before any
   authorization decision is made.

Normalization is not optional and applies equally to public and protected
routes. The gateway never forwards a path whose normalized form differs from
the form it authorized.

### §2.2 Matching

Routes are evaluated in declaration order. The first route that matches wins.

A route matches when its `prefix` equals the normalized path, or when the
normalized path continues with `/` after the prefix. Matching is on path
**segment boundaries**: prefix `/public` matches `/public` and
`/public/a`, but not `/publicity` or `/publicx/a`. Comparison is
case-sensitive.

A route's own prefix is treated as already normalized; configuration that
contains `.` or `..` segments, or percent escapes, is rejected at load time.

If no route matches, the request is served by `defaultRoute`. If
`defaultRoute` is `null`, the gateway answers `404 not_found`.

---

## §3 Scope evaluation

### §3.1 Scope syntax

A scope is a colon-delimited list of one or more non-empty segments, for
example `orders:read`, `orders:read:all`, `admin:export`. Comparison is
case-sensitive.

### §3.2 Granted vs required

A call is authorized when the caller holds **every** scope the route
declares. A granted scope satisfies a required scope if and only if their
segment sequences are equal:

- `orders:read` satisfies `orders:read`.
- `orders:readonly` does **not** satisfy `orders:read`.
- `orders` does **not** satisfy `orders:read`.
- `orders:read:all` does **not** satisfy `orders:read`.

A longer granted scope is not a superset of a shorter required scope, and a
shorter granted scope is not a prefix grant. No implicit wildcard exists:
`*`, `orders:*` and similar values carry no special meaning and match only
the literal scope of the same name.

### §3.3 Reading granted scopes

Granted scopes come from the verified token's `scope` claim, which is either
an array of strings or a space-delimited string. Non-string entries are
ignored. A token with no `scope` claim grants no scopes.

An authenticated caller that lacks any required scope is rejected with
`403 insufficient_scope`; the response lists the missing scopes and the
request is not forwarded.

---

## §4 Token validation

### §4.1 Structure

The token must be a compact JWS with three segments: base64url header,
base64url payload, base64url signature. The header and payload must decode to
JSON objects. Anything else is rejected as `malformed`.

### §4.2 Signature algorithm

The header must carry an `alg` value. `alg` must be a member of the
configured `token.algorithms` set, which is `["RS256"]` in the shipped
configuration. Any other value — including `RS512`, `PS256`, `HS256` and
`none` — is rejected. The gateway must not accept an algorithm merely because
the configured key is capable of producing that signature.

### §4.3 Key selection

The header must carry a non-empty `kid`. The `kid` must identify a key in the
loaded JWKS. A token whose `kid` is absent, empty, or not present in the JWKS
is rejected; the gateway must not substitute another key. Exactly one key is
used for verification: the one named by `kid`.

### §4.4 Registered claims

The following claims are validated after the signature verifies.

| Claim | Rule |
|---|---|
| `iss` | Required. Must equal the configured `token.issuer` exactly. |
| `aud` | Required. Must contain the configured `token.audience` (a token may list several audiences). |
| `exp` | Required. Must be a NumericDate. The token is valid while `now <= exp + tolerance`. |
| `nbf` | Optional. When present it must be a NumericDate and the token is valid only while `now >= nbf - tolerance`. |
| `sub` | Required. Must be a non-empty string; it becomes `X-Auth-Sub`. |
| `roles` | Optional. When present it must be an array of strings; non-string entries are ignored. |

`tolerance` is `token.clockToleranceSeconds` from configuration — **30
seconds** in the shipped configuration. The tolerance is the only permitted
leeway on `exp` and `nbf`; it is not a validity extension. A token expiring
more than 30 seconds ago is expired. A token whose `nbf` is more than 30
seconds in the future is not yet valid.

Malformed claim types (for example a string `exp`, or an `aud` that is not a
string or array of strings) are rejected rather than coerced.

Rejections use `expired` for `exp`, `not_yet_valid` for `nbf`, and `claims`
for `iss`, `aud`, `sub` and structural claim problems. The distinction is
diagnostic only; all of them produce `401`.

### §4.5 What a verified token yields

A verified token yields exactly two values for downstream use: `sub` and the
list of string `roles`. `scope` is used only for the decision in `§3` and is
not forwarded.

---

## §5 Identity headers

The upstream trusts identity headers and performs no authentication of its
own. The gateway is therefore the only party allowed to set them.

### §5.1 Stripping

Before forwarding, the gateway removes **every** header whose name, compared
case-insensitively, is one of:

- `X-Auth-Sub`
- `X-Auth-Roles`
- `X-Auth-User`
- `X-Forwarded-User`

This list is a floor, not an exhaustive allowlist: any header the gateway
itself writes as identity must also be stripped from the caller's request.
Removal happens regardless of whether the route is public or protected, and
regardless of whether the request carries a token.

### §5.2 Writing

After stripping, and only for a request whose token verified successfully,
the gateway sets:

- `X-Auth-Sub` — the token `sub`.
- `X-Auth-Roles` — the token `roles`, comma-separated, without spaces.

On a public route, or on any route where no token was verified, the gateway
sets **no** identity headers. An upstream must never receive an
`X-Auth-*`/`X-Forwarded-User` header that did not originate from the gateway.

There is exactly one value per identity header on the upstream request. A
caller-supplied value can never appear alongside, before, or in place of the
gateway's value. Header names are matched case-insensitively, so
`x-auth-roles`, `X-Auth-Roles` and `X-AUTH-ROLES` are the same header.

### §5.3 Other headers

Hop-by-hop headers (`Connection`, `Keep-Alive`, `TE`, `Trailer`,
`Transfer-Encoding`, `Upgrade`, `Proxy-Authenticate`, `Proxy-Authorization`)
are not forwarded in either direction. `Host` is rewritten to the upstream
authority. All other caller headers are forwarded unchanged.

---

## §6 Failure responses

| Status | `code` | Condition |
|---|---|---|
| 400 | `bad_request` | Path cannot be normalized (§2.1) |
| 401 | `missing_token` | Protected route, no bearer token |
| 401 | `malformed` | Token is not a well-formed JWT |
| 401 | `signature` | Signature does not verify |
| 401 | `expired` | `exp` outside the accepted window |
| 401 | `not_yet_valid` | `nbf` outside the accepted window |
| 401 | `claims` | `alg`, `kid`, `iss`, `aud`, `sub` or claim types invalid |
| 403 | `insufficient_scope` | Authenticated but missing a required scope |
| 404 | `not_found` | No matching route and no default route |
| 502 | `bad_gateway` | Upstream unreachable or returned an invalid response |
| 504 | `gateway_timeout` | Upstream exceeded the request timeout |

`401` responses include `WWW-Authenticate: Bearer realm="hopguard"` and, for
an invalid token, `error="invalid_token"`. `403` responses include
`missing_scopes` as an array. Error bodies are JSON objects with an `error`
and `code` field and are never cached.
