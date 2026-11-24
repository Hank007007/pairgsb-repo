# Refresh grants

Refresh grants let a client obtain a fresh access token without re-presenting
the subject token.

## 1. Creation

A successful token exchange creates a refresh grant and returns its handle as
`refresh_token`. The grant records:

- the subject;
- the client and audience of the exchange;
- the scope of the access token that was issued;
- an expiry.

## 2. Rotation

Redeeming a refresh grant rotates it:

- the response contains a new refresh handle;
- the presented handle is invalidated immediately and cannot be redeemed again;
- redeeming an invalidated handle fails with `invalid_grant`.

Rotation happens on every successful redemption, whether or not the requested
scope changed.

## 3. Scope of a rotated grant

The new grant records the scope of the access token issued by the redemption.
The grant that created a reduced-scope access token carries that reduced scope;
redeeming it cannot produce a token with broader scope than the grant records.

A refresh grant's scope is the ceiling for what a redemption may request. A
redemption may ask for less scope; it may not ask for more.

## 4. Errors

| Situation | `error` |
|---|---|
| Unknown handle | `invalid_grant` |
| Expired handle | `invalid_grant` |
| Handle already redeemed | `invalid_grant` |
| Requested scope not covered by the grant | `invalid_scope` |

## 5. Expiry

A rotated grant keeps an expiry no later than the grant it replaced. It does
not extend the lifetime of the refresh family.
