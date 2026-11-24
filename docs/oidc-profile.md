# OIDC profile for noncebind

noncebind is the shared OpenID Connect login component used by the internal
back-office applications. Every provider integrated with it must be able to
serve a login that is at least as strict as this profile. This document states
the requirements the component is expected to meet; it does not describe how
the component is organised internally.

## 1. Providers and algorithms

### 1.1 Provider configuration

Each provider is registered with a stable name, an issuer, a client id and
secret, an audience, the algorithms it may use for ID token signatures, the
authorization and token endpoints, the JWKS URI, and the redirect URI used for
that provider. Providers do not share keys, issuers, or client registrations.

### 1.2 Signature algorithms

The set of algorithms a provider may use is fixed in that provider's
configuration and is limited to `RS256` and `ES256`. The `alg` value carried in
the token header is an untrusted declaration: it selects how the signature is
to be checked only if the configured algorithm set permits it.

- Tokens whose header declares `none` are rejected, whatever the provider
  configuration says.
- Tokens whose header declares `HS256` are rejected, whatever the provider
  configuration says. A provider public key is never used as an HMAC secret.
- A token signed with an algorithm the provider configuration does not list is
  rejected even when its signature is cryptographically valid.
- A token must be accepted when it is signed with an algorithm the provider
  configuration lists and is otherwise valid.

The `kid` header names the key that signed the token. Key selection is
described in `jwks.md`.

## 2. Claim validation

### 2.1 Issuer

The `iss` claim must equal the provider's configured issuer exactly. No prefix,
suffix, case, or normalisation tolerance applies. An issuer that differs from
the configured value by a single character is invalid.

### 2.2 Audience and authorized party

The `aud` claim must contain the client id this component is registered under
with that provider. `aud` may be a single string or an array of strings.

When `aud` is an array with more than one entry, the `azp` claim must be present
and must equal the client id. A token whose audience contains the client id but
whose `azp` names a different party is invalid. A token with several audiences
and no `azp` is invalid.

### 2.3 Expiry and issued-at

The `exp` claim is required and is compared against the current time. Clock
differences between noncebind and the provider are tolerated up to 120 seconds:
a token whose `exp` passed 60 seconds ago is still accepted, while one whose
`exp` passed 200 seconds ago is rejected.

### 2.4 Issued nonce

When a nonce was sent with the authorization request, the ID token's `nonce`
claim must be present and must equal that nonce exactly. The comparison is
described further in `login-binding.md`.

## 3. Authentication age

When the login transaction requests `max_age`, the ID token must carry
`auth_time`, and the time elapsed since the end-user authentication must not
exceed `max_age`. Clock differences of up to 120 seconds between noncebind and
the provider are tolerated for this comparison.

A requested `max_age` must actually reach ID token validation; a request that
is silently ignored does not satisfy this section.

## 4. Access token binding

When an access token is issued alongside the ID token, the ID token's `at_hash`
claim must be present and must match the access token. The hash is computed
over the ASCII bytes of the access token with the hash function associated with
the ID token's signature algorithm (SHA-256 for `RS256` and `ES256`) and the
left-most half of the digest is base64url-encoded without padding.

A missing or mismatching `at_hash` makes the ID token invalid.
