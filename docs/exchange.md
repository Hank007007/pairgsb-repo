# Token exchange endpoint

This document specifies the behaviour of `POST /oauth/token` for the token
exchange grant and for refresh grants. It describes the required behaviour, not
a particular implementation.

## 1. Request parameters

| Parameter | Required | Meaning |
|---|---|---|
| `grant_type` | yes | `urn:ietf:params:oauth:grant-type:token-exchange`, or `refresh_token` for a refresh grant. |
| `client_id` | yes | The calling service. It must exist in the client registration table. |
| `subject_token` | yes | The token representing the user on whose behalf the exchange happens. |
| `subject_token_type` | yes | The type of `subject_token`. Must be one of the token types the deployment accepts. |
| `requested_scope` | no | Whitespace-separated scope request. When absent or empty, the subject token's granted scope is used. |
| `actor_token` | no | The token of the service acting on behalf of the subject. |
| `actor_token_type` | no | The type of `actor_token`. When supplied it must be an accepted token type. |
| `mode` | no | `delegation` or `impersonation`. When absent, the mode is inferred from `actor_token`: present means delegation, absent means impersonation. |
| `ttl` | no | Requested lifetime in seconds. Must be a positive integer when supplied. |
| `refresh_token` | for refresh grants | The refresh handle being redeemed. |

Parameters that are present but malformed (an unknown `grant_type`, a
non-integer `ttl`, an unrecognised token type, an explicit mode that conflicts
with the supplied tokens) are rejected with `invalid_request`.

## 2. Exchange

### 2.1 Subject token validation

Before any scope work, the subject token must be verified:

- its signature must be valid;
- its issuer must equal the deployment issuer;
- its audience must equal the calling client's registered audience;
- it must not be expired at the time of the request;
- it must identify a subject.

A subject token that fails any of these checks makes the request fail with
`invalid_request`. No token is issued.

### 2.2 Result scope

The issued token's scope is the intersection of three sets:

1. the scopes requested by the caller (`requested_scope`, or the subject
   token's granted scope when the request omits it);
2. the scopes actually granted to the subject token;
3. the scopes the calling client is registered to receive.

All three sets are compared only after scope normalisation, alias resolution,
and wildcard handling as described in `scopes.md`.

Every requested scope must be available in both the subject grant and the
client registration. If any requested scope is not available, the request fails
with `invalid_scope` and **no access token and no refresh grant are created**.
The response must not silently narrow the request.

The issued token lists the normalised requested scopes, in the normalised
(canonical) form, with duplicates removed and in a stable order.

### 2.3 Delegation and impersonation

Impersonation (no `actor_token`) issues a token that speaks purely for the
subject. The result carries no `act` claim.

Delegation (`actor_token` present, or `mode=delegation`) issues a token for the
subject that records the acting service:

- `actor_token` is required when `mode=delegation`; a delegation request
  without it fails with `invalid_request`;
- `actor_token` is not allowed when `mode=impersonation`; such a request fails
  with `invalid_request`;
- the actor token must pass the same signature, issuer, expiry, and subject
  checks as the subject token; failure is `invalid_request`;
- the subject token's `may_act` claim must permit the actor. If the subject
  token carries no `may_act`, or `may_act` does not name the actor, the request
  fails with `access_denied`;
- the issued token carries an `act` claim describing the actor, following
  `delegation.md`.

### 2.4 Token lifetime

The issued token's lifetime is the smallest of:

1. the subject token's remaining lifetime;
2. the requested `ttl`, or the deployment default when no `ttl` is supplied;
3. the deployment's maximum token lifetime.

`expires_in` in the response equals that lifetime. A subject token with less
than the requested lifetime therefore shortens the result; a `ttl` above the
deployment maximum is capped.

### 2.5 Refresh grant creation

A successful exchange creates a refresh grant for the subject, scoped to the
scope of the issued token, and returns its handle as `refresh_token`. The grant
is bound to the client and audience of the exchange.

## 3. Error responses

Errors use the OAuth error body shape:

```json
{"error": "invalid_scope", "error_description": "..."}
```

| Situation | `error` | HTTP status |
|---|---|---|
| Malformed request, bad token type, unverifiable subject/actor token, unknown client, delegation mode without actor, impersonation mode with actor | `invalid_request` | 400 |
| A requested scope is not available from the subject grant or the client registration | `invalid_scope` | 400 |
| Delegation not permitted by `may_act`, or an `act` chain that would exceed the allowed depth | `access_denied` | 403 |
| Unknown, expired, or already used refresh grant | `invalid_grant` | 400 |

Error bodies never contain stack traces, internal exception representations, or
other implementation detail. `error_description` is a short, safe human-readable
message.

## 4. Refresh

A refresh request carries `grant_type=refresh_token`, `client_id`, and
`refresh_token`. `requested_scope` is optional.

- The refresh grant must exist, must not be expired, and must not have been
  redeemed before. Otherwise the request fails with `invalid_grant`.
- When `requested_scope` is supplied it must be covered by the grant's current
  scope, using the comparison rules in `scopes.md`; otherwise the request fails
  with `invalid_scope`.
- When `requested_scope` is absent, the grant's current scope is used.
- The issued access token carries the resulting scope, and the response
  contains a new refresh handle as described in `grants.md`.
