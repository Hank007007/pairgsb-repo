# Token introspection

The resource server learns everything it needs about an access token from the
authorization server's introspection endpoint (RFC 7662). This document fixes
the request format, the response fields that matter, and the caching rules.

## 1. Request

`POST` to the configured introspection URL with:

```
Content-Type: application/x-www-form-urlencoded
Authorization: Basic base64(client_id:client_secret)

token=<the opaque access token>&token_type_hint=access_token
```

The access token is sent exactly as received. It is never logged, never put in
a URL, and never used as a cache key in clear.

## 2. Response

A `200` response with a JSON object. Relevant fields:

| Field | Meaning |
|---|---|
| `active` | boolean. Whether the token is currently valid. |
| `exp` | NumericDate. Expiry of the token. Absent for tokens without expiry, but always present for tokens issued for this resource server. |
| `sub` | the subject the token was issued to. |
| `scope` | space-delimited scope string. |
| `cnf.jkt` | the RFC 7638 fingerprint of the client's key (see `docs/dpop-profile.md` §7). |

### 2.1 Decision

A token is authenticated if and only if:

- the HTTP call succeeded and returned a JSON object,
- `active` is `true`,
- `exp` is absent or strictly greater than the current time,
- `cnf.jkt` is present.

Anything else is unauthenticated. Note that `active: false` is a deliberate
answer from the authorization server, not an error condition to be logged and
overridden. An unparseable response, a network error, a `5xx`, or a timeout are
errors; the token must be treated as unauthenticated and the request rejected.
There is no fail-open mode.

The principal attached to the request context is the `sub` value. Scope and
token metadata may be attached alongside it.

## 3. Caching

Introspection is a network round trip on the request path, so results may be
cached. The following constraints are mandatory:

1. **Key.** The cache key contains at least a cryptographic hash of the access
   token (SHA-256, hex or base64url; the choice is internal). The raw token
   string must not be used as a map key.
2. **Lifetime.** An entry lives at most `min(configured_ttl, exp - now)`. When
   `exp` is absent or farther away than the configured TTL, the TTL applies.
   When `exp` is closer, the entry expires with the token. A cached result must
   never outlive the token itself.
3. **Content.** An entry stores the complete positive result: subject, scope,
   and the `cnf.jkt` binding. A cache hit must be indistinguishable from a
   fresh introspection for every consumer, including the token binding check.
4. **Revocation.** Because positive results are cached, revocation takes effect
   at the latest when the entry expires. This window is bounded by
   `min(ttl, remaining lifetime)` and must not be extended by re-caching on a
   hit (a hit never resets the lifetime).
5. **Negative results.** `active: false` may be cached for a short period, but
   never as a positive entry, and never for longer than the configured TTL. It
   must never be transformed into a success.

The default configured TTL is 10 minutes. Deployments that need faster
revocation lower it; correctness never depends on it.

## 4. Failure handling

If the authorization server cannot be reached, the request is rejected with
`401` and `WWW-Authenticate: DPoP error=invalid_token`. A stale positive cache
entry may be used only if it is still within its computed lifetime; a cache
implementation must not extend entries on failure. Failures are not cached.
