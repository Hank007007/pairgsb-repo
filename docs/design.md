# proofbind design

This document describes the behaviour the resource server is expected to
implement. It is written from the outside in: what a caller sees, what the
authorization server promises, and where the resource server's obligations
begin and end. It deliberately does not describe code structure beyond the
package boundaries, and it gives no implementation.

## 1. Scope

`proofbind` is the token-validation middleware in front of an internal resource
server. Every request to a protected route carries two credentials:

- `Authorization: DPoP <access token>` — an OAuth access token issued by the
  authorization server, restricted to the `dpop` token type.
- `DPoP: <proof>` — a JWT signed by the client's private key.

The two credentials are bound to each other. A valid access token is necessary
but not sufficient: the proof must demonstrate possession of the key that the
token was issued to, and it must be bound to this particular request.

The middleware either attaches an authenticated principal to the request
context and passes the request on, or it rejects the request. There is no
third outcome, and a rejected request never reaches a handler.

## 2. Protocol inputs

### 2.1 Access token

The access token is an opaque string. It is not parsed by the resource server;
it is presented to the authorization server for introspection
(`docs/introspection.md`). The only thing the resource server relies on is the
introspection response, which carries at least `active`, and for sender-
constrained tokens also `exp`, `sub` and `cnf.jkt`.

### 2.2 Proof JWT

The proof is a JWT with three base64url segments. Its header contains:

| Member | Requirement |
|---|---|
| `typ` | exactly `dpop+jwt`, case-sensitive |
| `alg` | the signing algorithm; asymmetric only (ES256 or RS256), never `none` |
| `jwk` | the public key, as a JSON Web Key. Private key material must not be present. |

Its payload contains `jti`, `htm`, `htu`, `iat`, and normally `exp` and `ath`.
`docs/dpop-profile.md` states the exact semantics of each claim.

### 2.3 The binding

Two fingerprints tie the three protocol inputs together:

- `jkt` of the proof header `jwk` — computed per RFC 7638 over that key alone.
- `cnf.jkt` from introspection — the fingerprint the authorization server
  recorded when it bound the token.

These two values must be equal. Equality is byte-for-byte: the base64url text
is compared as text, with no case folding, trimming or other normalization.
The fingerprint of the key in the *proof header* is the only one that matters;
a `jwk` claim elsewhere in the proof is not part of the protocol and must not
be used as a substitute.

## 3. Request processing

The middleware processes a request in this order. A failure at any step ends
processing with `401 Unauthorized` (plus a `WWW-Authenticate: DPoP error=...`
challenge where a protocol error code is available). No step may be skipped on
the strength of an earlier step succeeding.

1. **Extract credentials.** Both `Authorization` and `DPoP` headers must be
   present and well-formed. Missing, duplicated or malformed credentials are
   rejected.
2. **Verify the proof signature.** Parse the JWT, resolve the public key from
   the header `jwk`, and verify the signature. The algorithm named in the
   header must be an asymmetric algorithm the server accepts, and it must be
   the algorithm actually used to verify. Symmetric algorithms and `none` are
   rejected. `typ` must be exactly `dpop+jwt`.
3. **Check freshness.** `iat` must be within 60 seconds of server time in
   either direction. `exp`, when present, must not have passed; when absent the
   proof is not acceptable. A proof that is old, has no expiry, or has expired
   is rejected regardless of how it is used later.
4. **Bind the proof to the request.** `htm` must equal the request method
   exactly, compared case-sensitively. `htu` must equal the request URL after
   removal of query and fragment (see `docs/dpop-profile.md` §4), again exactly.
5. **Bind the proof to the token.** `ath` must equal the base64url-encoded
   SHA-256 of the access token exactly as presented in the `Authorization`
   header, with base64url padding omitted.
6. **Check for replay.** The tuple (proof key fingerprint, `jti`) identifies a
   proof. Within the validity window of a proof, each such tuple may be
   accepted exactly once. Different clients using the same `jti` value must not
   interfere with each other, and two requests carrying the *same* proof must
   not both be accepted, even when they arrive concurrently.
7. **Introspect the token.** Ask the authorization server. The token is
   authenticated only if the response is `active: true` and, when present,
   `exp` is still in the future. `active: false` is a rejection — not a warning
   and not an invitation to continue. Any error reaching the authorization
   server, or any response that cannot be understood, is a rejection.
8. **Enforce the token binding.** Compute the RFC 7638 fingerprint of the proof
   header `jwk` and compare it to `cnf.jkt`. A token without `cnf.jkt` cannot
   be used with this middleware. A fingerprint that cannot be computed (for
   instance, an unsupported key type) is a rejection, never a skip.
9. **Attach the principal.** Only after all of the above: place the subject
   (and other useful token attributes) into the request context and call the
   next handler.

The ordering of steps 6 and 7 is not observable to a correct client as long as
both are enforced before any handler runs. Implementations may reorder them to
avoid contacting the authorization server for replaying proofs, provided the
outcome is unchanged.

## 4. Caching

The resource server may cache introspection results. When it does:

- The cache key must include at least a hash of the access token. Raw token
  strings are bearer secrets and must not be used as map keys, log fields or
  anything else that outlives the request.
- The lifetime of a cache entry must not exceed the remaining lifetime of the
  token according to its `exp`. If `exp` is absent, the entry must not outlive
  the configured maximum TTL.
- A cache entry stores the whole authentication decision, not only the boolean
  `active` value. In particular the `cnf.jkt` binding and the subject must
  survive a cache round-trip; a cache hit must yield exactly what a fresh
  introspection would have yielded.
- A cached `active: true` result may continue to be honoured until it expires,
  so revocation is eventually consistent within the cached lifetime. That is
  an accepted trade-off; serving a token past its `exp` is not.

The replay cache is separate and is not an optimisation: it is part of the
protocol. Its entries must be retained for at least the lifetime of the proof
they describe, and pruning must never remove an entry that is still inside its
validity window.

## 5. Algorithms and key rotation

The authorization server rotates between ES256 and RS256 without notice
(`docs/operations.md`). A proof signed with either algorithm must verify, for
as long as the corresponding public key is carried in the proof header. The
server must not pin a single algorithm, must not trust the `alg` header blindly
without confirming the key is of the matching type, and must support computing
RFC 7638 fingerprints for both EC (P-256) and RSA keys, since the token
binding check has to work in both cases.

## 6. Failure reporting

Every rejection is a `401`. Protocol-specific failures should carry a
`WWW-Authenticate: DPoP` challenge with an `error` parameter
(`invalid_dpop_proof`, `invalid_token`, `use_dpop_nonce` if nonces are ever
introduced, and so on). The response body must not disclose whether the token
exists, whether it was revoked, or which check failed beyond the coarse error
code. Replay of a valid proof is reported like any other invalid proof.

None of the checks above is advisory. There is no debug mode, header, or
configuration flag that downgrades a failed check to a warning.
