# Login binding for noncebind

A login is a three-step exchange: noncebind sends the browser to the provider,
the provider sends it back with an authorization code, and noncebind obtains
and validates a token set. The steps are tied together by the login
transaction.

## 1. Login transaction

Starting a login creates one transaction. It records:

- a `state` value and a `nonce` value, both generated with a cryptographically
  secure random source and both unpredictable;
- the provider the login is for;
- the redirect URI the browser will return to;
- the requested `max_age`, if any;
- when the transaction was created.

The authorization request carries the `state` and `nonce` of exactly that
transaction.

A transaction is single-use. The first callback that presents its `state`
consumes it. After that, the transaction is no longer retrievable: a second
callback with the same `state` is rejected, and a lookup of the used `state`
finds nothing.

The `redirect_uri` presented on the callback must be compared with the one
recorded on the transaction and must match exactly. A callback that presents a
different redirect URI is rejected before any token exchange takes place.

## 2. Nonce

The `nonce` is sent in the authorization request and returned inside the ID
token as the `nonce` claim.

- The ID token's `nonce` must equal the nonce recorded on the transaction that
  the callback consumed.
- A missing nonce claim is rejected.
- A nonce may be used once. A second presentation of an already-consumed
  transaction is rejected by §1 and never reaches nonce comparison.

## 3. Session transition

The component keeps anonymous sessions for visitors who have not logged in and
authenticated sessions for users who have.

- An anonymous session id is not an authenticated session id. Possession of a
  pre-login id must not grant authenticated access.
- A successful login creates a new session with a new id. The cookie set on the
  callback response carries the new id, which differs from the anonymous id the
  browser presented.
- The anonymous session is discarded at the moment the authenticated session
  is created. The old id is not usable afterwards, whether presented as a
  cookie or looked up directly.
- The authenticated session records the subject and claims from the validated
  ID token.
- Only a fully validated ID token may create an authenticated session. A token
  that fails any check in `oidc-profile.md` does not promote a session.
- A failure while obtaining the token set produces an error response with a
  4xx status. A failed login does not return a success status or an empty
  document that a browser would treat as a completed login.
