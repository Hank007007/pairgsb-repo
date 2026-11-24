# JWKS handling for noncebind

Each provider publishes its signing keys at the JWKS URI recorded in its
configuration. Keys rotate: a provider may publish a new key with a new `kid`
while the old key remains valid for a while, and it may later withdraw the old
key.

## 1. Retrieval

A JWKS document is a JSON object with a `keys` array. Each entry is a JWK with
a `kty` of `RSA` or `EC`, a `kid`, and the public key parameters. Documents that
cannot be retrieved, are not JSON, have no `keys` array, or contain no usable
key are errors.

## 2. Caching and refresh

Retrieving a JWKS document is an outbound request to the identity provider and
is subject to rate limits and availability.

- A successful JWKS response is cached per JWKS URI for at least five minutes.
- While a cached document is fresh, token validation uses it and does not fetch
  the JWKS again, even for repeated validations.
- A fetch is performed when the cached document is older than the allowed
  freshness window, or when the token names a `kid` that is not present in the
  cached document.
- Consecutive fetches are separated by a minimum interval. A burst of tokens
  naming an unknown `kid` must not turn into a burst of outbound requests.
- If the minimum interval has not elapsed, the request is answered from the
  cached document rather than by fetching again.

## 3. Key selection

- The token's `kid` header selects the candidate key. Only a key whose `kid`
  matches exactly may be used to verify that token.
- A token whose `kid` is not present in the JWKS is rejected. The component does
  not verify the token against every key in the document in the hope that one
  of them happens to validate.
- When the `kid` matches a key, only that key is used. Other keys in the
  document are not tried as fallbacks.
- A token with no `kid` cannot be attributed to a key and is rejected.
