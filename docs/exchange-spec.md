# ScopeBraid Token Exchange — Design Specification

Status: normative for the implementation under `scopebraid/`.
Audience: engineers working on the exchange service.

ScopeBraid is an internal token exchange service modelled on OAuth 2.0 Token
Exchange (RFC 8693). A registered internal service (the *caller*) presents an
end-user access token (the *subject token*) and receives a new, narrower
access token intended for exactly one downstream service.

This document describes required behaviour only. It deliberately does not
prescribe data structures or algorithms.

---

## §1 Scope narrowing

An issued token's `scope` is the intersection of three sets, all of which
must participate:

| Set | Origin |
|---|---|
| requested | the `scope` parameter of the exchange request |
| subject | the `scope` claim carried by the subject token |
| registered | the scopes recorded in the caller's client registration |

A scope token is issued if and only if it is granted by all three sets. A
missing set contributes nothing: if the subject token carries no scope, the
issued token carries no scope even if the request and the client registration
both name some. If the request omits `scope`, the result is the empty set —
the service never invents scope on the caller's behalf.

The requested set may name tokens the subject or the registration cannot
support. Such tokens are dropped from the result rather than causing the
whole exchange to fail. The result must never contain a token that any of the
three sets does not grant.

Scope values are space-delimited token lists. Duplicate tokens are collapsed
and order is not significant to the result.

## §2 Wildcards

A granted scope token may end in a wildcard:

- `files:*` stands for every token beginning with `files:`.
- A bare `*` stands for every token.

A wildcard covers a requested token only when the requested token begins with
the wildcard's literal prefix. Thus `files:*` covers `files:read` and
`files:write`, but not `admin:write`; `*` covers `admin:write`. Wildcards are
only meaningful on the granting side. A requested token containing a
wildcard is treated as a literal token for comparison.

The effective scope of an issued token is always expressed as concrete
tokens; a wildcard is never copied into an issued token's `scope` claim. When
several granted wildcards cover the same requested token, the token is issued
once.

## §3 Subject token types

Only an access token may serve as the subject of an exchange. The accepted
value of `subject_token_type` is:

```
urn:ietf:params:oauth:token-type:access_token
```

Any other value — including refresh tokens and ID tokens, however their
identifier is spelled — must be rejected with `invalid_request`. The same
rule applies to `requested_token_type`: only an access token may be
requested. A request that asks for any other kind of token is rejected with
`invalid_request`.

The service must not infer the type of the presented token from its contents
and must not silently accept a mismatched type declaration.

## §4 Audience

The `audience` parameter names the single downstream service the issued token
is for. It is required.

The subject token's `aud` claim may be a string or an array of strings. The
requested audience is acceptable only if it equals one of the subject token's
audience values. When `aud` is an array, every element is a candidate — not
just the first one.

Independently, the requested audience must appear in the audiences recorded
in the caller's client registration. A caller may only obtain tokens for
audiences it is registered to request; naming an internal service that is not
in its registration must be rejected.

Both checks are required. Failure of either is reported as `invalid_target`.

## §5 Delegation chain (`act`)

Every issued token records who is acting. The `act` claim is a JSON object
describing the delegation chain, with the most recent actor at the head:

```json
"act": {
  "sub": "svc-b",
  "jti": "<jti of the token svc-b presented>",
  "act": {
    "sub": "svc-a",
    "jti": "<jti of the token svc-a presented>",
    "act": { "sub": "svc-c", "jti": "..." }
  }
}
```

Requirements:

1. Each exchange appends a new link at the head of the chain. The existing
   chain from the subject token is preserved verbatim beneath the new link.
   It is never replaced.
2. Each link carries the actor's `client_id` as `sub` and the `jti` of the
   subject token that actor presented.
3. The chain has a maximum depth of three links. An exchange that would
   produce a fourth link must be rejected with `invalid_request`; no token is
   issued.
4. The chain from the subject token is carried into the new token unchanged
   in content and order.

A chain of links is also called the delegation path. Depth is counted in
links, so a token issued from a subject token with no `act` claim has a
one-link chain and is the first hop.

## §6 Revocation

Revocation is recorded per token identifier (`jti`) in the service's
revocation table.

A token is unusable if its own `jti` is revoked or if any token referenced
anywhere in its `act` chain has a revoked `jti`. Revoking an upstream token
therefore invalidates every token derived from it, at any depth, without
needing to enumerate the descendants.

The check applies both when validating an incoming subject token and when
validating a previously issued downstream token. A token whose chain contains
a revoked link must be rejected as revoked.

## §7 Lifetime and signing

An issued token's `exp` is the earlier of:

- the service's configured default lifetime from the time of issue, and
- the subject token's own expiry.

No token outlives the authority it was derived from. An already-expired
subject token is rejected before any other processing.

Issued tokens are signed with the service's HMAC key using the compact format
implemented in `scopebraid/tokens.py`. Claims `iss`, `sub`, `aud`, `scope`,
`client_id`, `token_use`, `iat`, `nbf`, `exp`, and `jti` are mandatory. The
`token_use` claim is `access`.

## §8 Error responses

Errors use the OAuth error shape:

```json
{"error": "invalid_request", "error_description": "..."}
```

| Condition | `error` |
|---|---|
| malformed, unsigned, or expired subject token | `invalid_request` |
| unsupported `subject_token_type` or `requested_token_type` | `invalid_request` |
| delegation chain would exceed three links | `invalid_request` |
| missing audience, audience mismatch, or unregistered audience | `invalid_target` |
| caller is not registered | `invalid_client` |

The `error_description` is informational and must not be relied on by
callers.

## §9 Invariants

- An issued token's scope is a subset of each of the three sources named in
  §1.
- An issued token is valid only for its single `aud` value.
- The `act` chain of an issued token contains exactly one more link than the
  subject token's chain, up to the three-link maximum.
- No issued token is usable after any token in its delegation chain is
  revoked.
