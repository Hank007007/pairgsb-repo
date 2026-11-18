# grantshift token service — normative behaviour

This document is the contract the service is expected to satisfy. It describes
observable behaviour only; how the code achieves it is up to the implementation.

Terminology used below:

- **grant** — one successful authorization, identified by `family_id`.
- **family** — every token minted from the same authorization-code redemption,
  including each rotation of the refresh token. A token exchange opens a new
  family (§7.3), so revoking a grant does not affect delegated tokens that were
  already minted from it.
- **actor** — the client that is asking to act on somebody's behalf.

## §1 Authorization endpoint

`POST /authorize` records a grant and returns a one-time code.

- `client_id` must be known. Unknown clients yield `invalid_client` (401).
- `scope` is a space-delimited list, required and non-empty. Duplicates are
  removed and the ordering is preserved.
- `audience` is required. It names the single resource server the resulting
  tokens are valid for.
- PKCE: `code_challenge_method` may be absent or `S256`. Any other value,
  including `plain`, is rejected with `invalid_request` (400).
  A challenge supplied without a method is treated as `S256`.
- `may_act`, when present, is an object with a string `sub`. It records which
  client is allowed to exchange a token from this grant for another (§7).

Codes are single-use and expire after `code_ttl` seconds.

## §2 Authorization-code exchange

`POST /token` with `grant_type=authorization_code`.

The request is checked in this order; the first failure wins:

1. client authentication (§8);
2. the code exists, was issued to this `client_id`, has not been consumed and
   has not expired (`invalid_grant`);
3. `redirect_uri` matches the value stored with the code, when one was stored;
4. PKCE (§3);
5. `audience`, when supplied, equals the audience recorded on the code.

On success the code is consumed, a new family is created, and an access token
plus a refresh token are returned. The refresh token is bound to the client
and to the audience.

## §3 PKCE verification

`S256` is the only accepted transformation. The verifier presented at the token
endpoint is ASCII, 43–128 characters long, and is verified by computing

```
BASE64URL-ENCODE( SHA256( ASCII( code_verifier ) ) )   # no padding
```

and comparing that value with the stored `code_challenge`. The raw
`code_verifier` is never compared with the challenge. A missing verifier, or a
mismatch, is `invalid_grant` (400). The `code_challenge_method` recorded at
authorization time determines the transformation that applies at redemption;
`S256` is the only value that can be recorded.

## §4 Scope at issuance

A token request may ask for a narrower scope than the grant allowed, never a
wider one.

- The **granted scope** is the scope approved at `/authorize`; later requests
  cannot change it.
- The **requested scope** (the optional `scope` field on `/token`) is a filter.
- The scope actually written into the token is the intersection of the two,
  in granted-scope order, with duplicates removed. Scope names that were not
  granted are dropped — they never appear in a token or in the response.

Applies uniformly to authorization-code exchange, refresh and exchange.

## §5 Refresh

`POST /token` with `grant_type=refresh_token`.

- The presented token must be a refresh token, must belong to the
  authenticated client, must still exist server-side, must not have been
  rotated before, and must not be revoked.
- The new pair is issued into the same family. The old refresh token is marked
  as rotated and cannot be used again.
- `audience` is **not** a request-time choice. The refreshed tokens carry the
  audience that was recorded when the refresh token was issued. A request that
  supplies a different `audience` is rejected with `invalid_grant` (400).
- Scope follows §4, using the refresh token's own scope as the granted scope.
- A rotated refresh token keeps the grant's `may_act`.

## §6 Revocation and lifetime

`POST /revoke` records a revocation and returns `{"revoked": true}`. Revoking
any token revokes its entire family: every access token and refresh token
issued from that grant stops being usable immediately, including access tokens
that were signed before the revocation and have not reached `exp`.

Therefore:

- `POST /introspect` answers `active: false` for any token whose family has a
  revocation record, as well as for unknown, malformed, or expired tokens.
- `GET /resource/me` answers 401 `invalid_token` for such tokens.
- Expiry is evaluated against `exp`; a token past `exp` is never active.
- Revocation is idempotent. Revoking a token that is unknown or already
  revoked still returns `{"revoked": true}` and does not error.

Families are identified by `family_id`, which every issued token carries in
its claims and in its server-side row.

## §7 Token exchange

`POST /token` with `grant_type=urn:ietf:params:oauth:grant-type:token-exchange`.

The `subject_token` must be a currently valid access token (§6). `audience` is
required and names the resource server the delegated token is for.

### §7.1 Authority to act

Delegation is only legal when the subject token names the requesting client as
its actor. Concretely:

- The subject token carries `may_act: {"sub": "<client_id>"}`, either because
  the grant recorded one or because a previous exchange passed one on.
- The authenticated `client_id` must equal that `may_act.sub`.
- A request from any other client is rejected with `invalid_grant` (400) —
  possessing a valid subject token is not by itself authority to act.
- A subject token with no `may_act` cannot be exchanged by anyone.

`may_act` may itself carry a nested `act` naming a further hop:
`may_act: {"sub": "svc-billing", "act": {"sub": "svc-reports"}}` means billing
may act, and the delegation may be passed on from billing to reports. The
requesting client must be named by the subject token's `may_act` — as its
`sub`, or as a level of the nested chain — and any other client is refused.
The `may_act` recorded on the exchanged token is the subject token's
`may_act`, carried over unchanged, so the grant's delegation graph is
preserved and cannot be extended by a request.

A client may repeat `may_act` in the exchange request only to restate the
value already carried by the subject token. It may not name a client the
grant did not allow; doing so is `invalid_grant`. When the field is omitted
the subject token's `may_act` is used.

### §7.2 Delegation chain

The `act` claim records who is acting. It is a nested chain, outermost actor
first:

```
"act": {"sub": "<current actor>", "act": {"sub": "<previous actor>", ...}}
```

- Every access token minted from an authorization-code exchange already
  carries `act: {"sub": "<client_id>"}` — the client that redeemed the code is
  the first actor.
- An exchange **prepends** the requesting client to the subject token's chain
  and keeps the subject token's `act` nested underneath it. The existing chain
  is never discarded, overwritten, or flattened to one level.
- Consequently a token that has passed through one delegation hop carries a
  two-level chain, and one that has passed through two hops carries a
  three-level chain (`reports → billing → portal`). Resource servers rely on
  the full chain to attribute actions.
- If the subject token has no `act`, the exchanged token's chain starts with
  the requesting client.
- `GET /resource/me` and `POST /introspect` must expose the chain unmodified.

### §7.3 Exchange result

The exchanged token is a fresh access token for `subject_token.sub`, for the
requested `audience`, with the narrowed scope from §4 and the audience taken
from the request. It starts a new family of its own and no refresh token is
issued. The response carries `issued_token_type` =
`urn:ietf:params:oauth:token-type:access_token`.

## §8 Client authentication

Confidential clients authenticate with `client_secret`; a wrong or missing
secret is `invalid_client` (401). Public clients have no secret and must not
send one; doing so is `invalid_client`. Every endpoint that takes a token or
code authenticates the caller first.

## §9 Token format

Tokens are HS256 JWTs signed with the service secret. Required claims:
`iss`, `sub`, `aud`, `client_id`, `scope`, `jti`, `family_id`, `token_type`
(`access_token` or `refresh_token`), `iat`, `exp`. `may_act` and `act` appear
when applicable. Every issued token also has a server-side row; a token with no
row is not active.
