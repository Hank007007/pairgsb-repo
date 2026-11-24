# noncebind design

noncebind is a library, not a web framework. It exposes plain functions and
small data objects so that the portal, the ops console, and the partner site
can each plug it into their own HTTP stack. This document describes the
intended behaviour of each layer and the boundaries between them.

## Layering

```
authorization request  ->  callback handler  ->  token endpoint  ->  ID token validation
                                                                        |
                                                                        v
                                                            JWKS cache and key selection
                                                                        |
                                                                        v
                                                              session store transition
```

Each layer has one job:

- `noncebind/providers.py` is data. It answers "what does this provider look
  like" and never performs I/O or validation.
- `noncebind/oidc/jwks.py` owns retrieval, caching, and the mapping from a
  token's `kid` to candidate keys.
- `noncebind/oidc/verify.py` owns everything about an ID token: which
  algorithm and key may verify it, and which claims must hold.
- `noncebind/login/transaction.py` owns the lifecycle of one login attempt.
- `noncebind/login/callback.py` owns the orchestration: consume the
  transaction, exchange the code, validate, then hand the identity to the
  session store. It makes no cryptographic decisions of its own.
- `noncebind/session/store.py` owns the two kinds of session and the transition
  between them.

## Login start

Starting a login allocates a transaction and builds the authorization URL from
it. The URL carries:

- `response_type=code`, the provider's client id, scope, and redirect URI;
- the transaction's `state` and `nonce`;
- `max_age` when the caller asked for one.

The `state` and `nonce` are independent, unpredictable values. They are stored
on the transaction so the callback can compare against them.

## Callback

The callback's job is to decide whether this response can be turned into a
session. In order:

1. The `state` parameter must identify a live transaction. The transaction is
   consumed at this point.
2. The callback's redirect URI must be identical to the one on the transaction.
3. The authorization code is exchanged at the provider's token endpoint, using
   the transaction's redirect URI.
4. The ID token is validated, using the transaction's nonce and the
   transaction's `max_age`, together with the access token from the token
   response.
5. Only then does the session store produce an authenticated session.

Every failure before step 5 yields an error status. A failure to obtain the
token set is a client-visible error (4xx), not a success page.

The callback does not duplicate validation logic. If a rule belongs to ID
token validation, it lives in `verify.py`; if it belongs to the transaction,
it lives in `transaction.py`.

## ID token validation

Validation is a sequence of independent checks; all must pass.

- The token must be a well-formed compact JWT with a JSON header and claim
  set.
- The header algorithm must be one the provider is configured for. The
  configured set is the only source of truth; the header cannot widen it.
  `none` and `HS256` are never acceptable, and a public key is never used as a
  symmetric secret.
- The signing key is the one named by `kid`; see `jwks.md`. Verification uses
  that key only.
- `iss` equals the provider issuer exactly.
- `aud` contains the client id. When `aud` lists more than one audience, `azp`
  must be present and equal to the client id.
- `exp` is in the future, allowing the configured clock skew.
- When a nonce is supplied for comparison, the token's `nonce` claim is present
  and equal.
- When `max_age` is supplied, `auth_time` is present and the elapsed time since
  authentication, allowing the configured clock skew, does not exceed
  `max_age`.
- When an access token is supplied, `at_hash` is present and matches the
  access token under the hash function of the token's algorithm.

`verify_id_token` returns the claim set only when every applicable check
passes. Callers are not expected to re-check claims afterwards.

## JWKS

`get_keys` returns the provider's current signing keys. `find_by_kid` returns
the key that matches the requested `kid`.

The cache is scoped per provider JWKS URI. A document obtained from the
provider remains usable for a freshness window of at least five minutes. While
it is fresh, no request is made to the provider.

A `kid` miss is the one situation that may trigger an early fetch, because the
provider may have rotated keys since the document was cached. That fetch is
rate-limited: an unknown `kid` repeated many times in quick succession results
in one fetch, not one per token. If a fetch does not resolve the `kid`, the
token is rejected; keys in the document are not tried in its place.

## Sessions

The session store holds two kinds of record:

- anonymous: created for a visitor before login, not authenticated;
- authenticated: created by a completed login, carrying the subject and claims
  from the ID token.

`create_anonymous` returns a fresh anonymous session. `promote` performs the
transition: it creates a new session record with a new identifier, copies the
validated identity onto it, and removes the anonymous record. The caller sets
the cookie from the session returned by `promote`, which is therefore never the
pre-login identifier.

A lookup for a removed identifier returns nothing. Holding the pre-login cookie
after a successful login grants no access.

## Errors

Each layer raises a typed error for its own failures
(`UnknownProviderError`, `JwksError`, `InvalidTokenError`,
`TransactionError`, `TokenExchangeError`, `SessionError`). The callback maps
them to status codes: bad or missing callback parameters and unusable
transactions are 400, failed token exchange is a 4xx, and failed ID token
validation is 401. Nothing is mapped to a 2xx success.
