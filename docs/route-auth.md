# Route authorization contract

This document is the normative description of how routeclaim must decide
whether a request may reach an upstream, and what the upstream is allowed to
observe about the caller. It describes required behavior only; it does not
prescribe an implementation.

Terminology used below:

- **request path** — the path component of the inbound request target, without
  the query string.
- **normalized path** — the result of applying §1 to the request path.
- **route** — one entry of the route table in `config/routes.yaml`.
- **gateway-owned headers** — the `x-auth-*` headers listed in §6.

---

## §1 Path normalization

Route matching operates on the normalized path, never on the raw request
target.

1. Percent-encoding is decoded before any other step. A path that cannot be
   decoded is rejected with `400`.
2. The path is segmented on `/`. Within a segment, empty segments and `.`
   segments are removed.
3. A `..` segment removes the preceding segment. A `..` that would escape the
   root is clamped at the root — it does not produce an error and it does not
   escape.
4. The result is a path that begins with `/` and contains no empty, `.` or
   `..` segments.
5. A trailing slash is not significant: `/api/admin` and `/api/admin/` denote
   the same path.
6. Literal path segments in the route table are compared after the same
   normalization, so a route declared as `/api/admin` also covers `/api/admin/`.
7. Path comparison is case-insensitive: `/API/Admin` matches the route
   `/api/admin`. Casing is not significant for selection, and a route's
   declared spelling is the one used when the route is reported.

Decoding happens exactly once per request. A path that already contains
encoded dot segments (`%2e%2e`), repeated slashes, or mixed separators must end
up at the same normalized path as its plainly written equivalent.

## §2 Route selection

1. A route matches when the normalized path equals the route's path or begins
   with the route's path followed by `/`. A route path is matched on segment
   boundaries: `/api/adm` does not match the route `/api/admin`, and `/apidocs`
   does not match the route `/api`. Comparison is case-insensitive (§1.7).
2. When more than one route matches, the route with the longest path wins.
   Declaration order in the route table is not significant for selection.
3. If no route matches, the request is answered with `404` and is not
   forwarded.

## §3 Method matching

1. A route matches a request method when the method is listed in the route's
   `methods`, compared case-insensitively.
2. The entry `*` in `methods` matches every method.
3. There is no implicit allowance for any method. A method that no matching
   route accepts is answered with `405` and is not forwarded. In particular
   `HEAD` and `OPTIONS` follow exactly the same path as every other method:
   they are matched against the route table, authenticated and authorized like
   any other request, and are never short-circuited ahead of route matching.

## §4 Authentication modes

Each route carries exactly one `auth` mode.

- `public` — no credential is required. Any credential presented by the caller
  is not used to derive identity.
- `jwt` — the request must carry a bearer token that verifies locally against
  the configured issuer and secret. A missing, malformed, expired or
  unverifiable token is rejected with `401`.
- `introspect` — the request must carry a bearer token, and the authorization
  service must report the token as active (§5). A missing token or an
  introspection result that is not active is rejected with `401`.

For `jwt` and `introspect`, the caller identity is derived from the token, not
from anything else the caller sent.

## §5 Token introspection and caching

The authorization service is the authority on whether an opaque token is
currently valid. Introspection responses carry at least `active`, `sub`,
`scope`, and — when the token has one — `exp` (seconds since the epoch).

1. A token may be served from cache only while every one of the following holds:
   - the cached record is younger than `introspection.cache_ttl_ms`;
   - the token's own `exp`, when present, has not passed.
   The effective lifetime of a cached record is therefore the smaller of the
   two.
2. Once a cached record expires it must not be used again; the next request
   carrying that token performs a fresh introspection.
3. Failure results are not shared between requests indefinitely. A token that
   introspection could not be completed for (transport error, timeout,
   non-2xx response, malformed body) must not be pinned to a failed outcome:
   the next request carrying it performs a fresh introspection.
4. A response with `active: false` is a definitive authorization decision for
   that request. It is not a cache miss, and it is not retried within the same
   request. A negative decision may be retained no longer than an active one
   (§5.1): once that window passes, the next request carrying the token must
   introspect again, so a token that is later re-activated is not kept
   rejected by an old negative record.
5. A token that the authorization service reports as inactive must be rejected
   with `401` on that request even if an earlier introspection reported it
   active.
6. Cache records are keyed per token *and* per authorization service endpoint
   and audience/issuer context, so a record obtained for one context is never
   served for another.

## §6 Identity propagation to upstreams

The upstream must only ever observe identity that the gateway itself produced
for this request.

1. The gateway sets `x-auth-user` (the token subject), `x-auth-scopes` (a
   space-separated, de-duplicated list), and — for `introspect` routes —
   `x-auth-token-id` (the authorization service's token identifier) when
   present.
2. These headers are always written by the gateway, replacing any value of the
   same name supplied by the caller. Caller-supplied values are never merged,
   appended to, or preferred.
3. `x-forwarded-user` and the other legacy identity headers listed below are
   stripped from the inbound request unconditionally, on every route including
   `public` routes:
   - `x-forwarded-user`
   - `x-auth-user`
   - `x-auth-scopes`
   - `x-auth-token-id`
   - `x-authenticated-user`
   - `x-remote-user`
4. Credentials are not forwarded to internal services. `authorization` and
   `cookie` are removed from the forwarded request on every route.
5. Header names are matched case-insensitively: `X-Auth-User`, `x-auth-user`
   and `X-AUTH-USER` are the same header for the purposes of (2) and (3).
6. On a `public` route the gateway sets no identity header at all; it still
   strips the headers in (3) and (4).

## §7 Scope evaluation

Scopes are opaque strings of the form `namespace:action`.

1. Comparison is exact on the full string. `docs:readonly` does not satisfy
   `docs:read`, and `docs:read` does not satisfy `docs:readonly`.
2. The single wildcard form `namespace:*` covers every action in that
   namespace. `admin:*` covers `admin:write`; it does not cover `billing:write`
   and it does not cover a scope from another namespace whose name merely
   starts with `admin`.
3. A bare `*` grants every scope.
4. Scope lists coming from a token are split on whitespace and treated as a
   set. Duplicates and ordering are not significant. The claim carrying the
   list may be spelled `scope`, `scopes`, `Scope` or `Scopes`: the claim name
   is matched case-insensitively. Scope values themselves are compared
   case-sensitively, as in §7.1.
5. A route with an empty `scopes` list requires no scope; it is satisfied by
   any authenticated caller.
6. Every scope listed on the route must be covered. A route requiring
   `[a:b, c:d]` is not satisfied by a token holding only `a:b`.
7. An authenticated caller whose scopes do not cover the route is rejected
   with `403`.

## §8 Failure responses

| Condition | Status |
|---|---|
| undecodable path | 400 |
| no route matches | 404 |
| method not accepted by the matched route | 405 |
| missing or invalid credential | 401 |
| token reported inactive | 401 |
| authenticated but missing a required scope | 403 |
| upstream unreachable | 502 |
| upstream exceeds the request timeout | 504 |

Gateway-generated error responses never include the caller's token.
