# Integration notes

Things the teams that call grantshift keep getting bitten by. Written from the
caller's side; § references point into `token-service.md`.

## Public vs confidential clients

`web-console` is a browser app: it has no secret, so it must send
`client_secret: null` (or omit the field). Sending an empty string is treated
as sending a secret and is rejected. Server-side services always have a secret.

## PKCE

The console generates a 43–128 character verifier, keeps it in memory, and
sends `code_challenge = base64url(sha256(verifier))` with
`code_challenge_method: "S256"`. The raw verifier only travels to `/token`.
Two teams have shipped the verifier itself as the challenge — do not do that:
`plain` is not accepted. See §3.

## Scope

Ask for what you need on `/authorize`; the grant records exactly that. On a
later `/token` call you may repeat a subset of it, and you get that subset
back. Asking for more than the grant approved does not error — the extra names
are simply not issued, and the response `scope` tells you what you actually
got. Do not assume the `scope` you sent equals the `scope` you received.

`scope` is one string with single spaces, never a list and never comma
separated.

## Audience

Every token is for exactly one resource server, named in `aud`. Billing calls
orders with `audience: "svc-orders"`; if the orders API sees `aud ===
"svc-orders"` it may trust the token, otherwise it must answer 401.

Refresh does not let you change the audience: the new pair stays on the
audience of the refresh token. If you need a token for another service, use
token exchange, not refresh. See §5.

## Refresh tokens

Always store the newest refresh token. The previous one is dead the moment the
new pair comes back — retrying a rotation with the old value returns
`invalid_grant`. Rotating does not widen or change the grant.

## Revocation

Revoking is family-wide and immediate. If a user logs out, revoke the refresh
token: their outstanding access tokens go dark at the same instant, even
though the JWT still verifies. Resource servers must not trust an
un-expired JWT on its own; either call `/introspect` or rely on grantshift's
`/resource/me`. See §6.

Revoking something twice, or revoking a token you no longer have, is not an
error.

## Delegation

A service can only exchange a subject token if that token names it as the
actor: the authorization request must have carried `may_act: {"sub": "<your
client id>"}`. Tokens without that claim are not delegable by anyone. This is
the one place where holding a valid token is deliberately not enough.

`may_act` can also carry a nested `act` saying who may act *after* you
(`may_act: {"sub": "you", "act": {"sub": "the-next-service"}}`). The chain is
carried over unchanged by each exchange, so delegation can be passed down it
but never widened.

When you exchange, the resulting token's `act` is a nested chain, not a single
name. If billing exchanges a token that the portal obtained, `act` reads
`{"sub": "svc-billing", "act": {"sub": "svc-portal"}}`; a second hop nests
again — three levels after two hops. Log the whole chain before you attribute
an action, and never rewrite it when relaying the token. Two services have
silently dropped the inner object and lost audit trails that way. See §7.2.

## Introspection

`/introspect` answers with the RFC 7662 fields. `active` is the only field you
may rely on being present when it is false. Tokens are typed: an access token
and a refresh token from the same grant both introspect, so check
`token_type` before treating one as the other.

## Error handling

Errors are `{"error": "...", "error_description": "..."}`. `invalid_client`
and `invalid_token` are 401; `invalid_request`, `invalid_grant` and
`unsupported_grant_type` are 400. Do not retry a 400 with the same body.

## Clock skew

Access tokens live 15 minutes, refresh tokens 24 hours, codes 5 minutes. A
small negative skew is tolerated on verification, but do not hold a token past
`exp` and expect `/introspect` to keep saying it is active.
