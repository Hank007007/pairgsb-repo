# DPoP proof profile

This document fixes the profile of DPoP proofs accepted by the resource
server. It is a strict subset of the DPoP specification as used internally.
Where this document and the external specification disagree, this document
wins.

## 1. Transport

Every protected request carries exactly one proof, in the `DPoP` request
header. The header value is a compact JWS: three base64url segments separated
by `.`, with no whitespace. The `Authorization` header is
`DPoP <access-token>` (scheme case-insensitive, single space, one token).

## 2. JOSE header

| Member | Rule |
|---|---|
| `typ` | MUST be the string `dpop+jwt`. This comparison is case-sensitive and exact. |
| `alg` | MUST be `ES256` or `RS256`. `none`, HS* and any other algorithm are rejected. |
| `jwk` | MUST be a JSON object holding an asymmetric public key in JWK form. Private members (`d`, `p`, `q`, `dp`, `dq`, `qi`, `k`) MUST be absent; their presence is a rejection. |

Additional header members are permitted and ignored, except that they must
never influence which key or algorithm is used for verification.

The key used to verify the signature is the key in the `jwk` header member of
*this* proof. There is no key registry, no `kid` lookup, and no fallback to a
key named anywhere else in the JWT.

## 3. Claims

All members are JSON strings; `iat` and `exp` are NumericDate values (seconds
since the Unix epoch).

| Claim | Rule |
|---|---|
| `jti` | MUST be present and non-empty. Uniqueness is scoped to `(jkt, jti)` as described in §6. |
| `htm` | MUST equal the HTTP method of the request, uppercase, compared case-sensitively. |
| `htu` | MUST equal the target URI after the normalization in §4. |
| `iat` | MUST be present. Server time minus `iat` MUST be in `[-60, +60]` seconds. |
| `exp` | MUST be present. The proof is invalid at or after `exp`. |
| `ath` | MUST be present and equal to `base64url(SHA-256(access_token))` without padding. |

A proof missing any of `jti`, `htm`, `htu`, `iat`, `exp` or `ath` is rejected.
Additional claims (`nonce` and so on) may be present; if the resource server
does not implement nonces it ignores them.

## 4. `htu` normalization

`htu` is a URI. The resource server computes the expected value from the
incoming request as follows:

1. Start from the scheme, host and path as the server sees them
   (`https://host/path`). Scheme and host are lowercased; the path is left
   exactly as received.
2. Drop any query string (everything from `?` onward).
3. Drop any fragment (everything from `#` onward).
4. Drop a trailing `:443` on an `https` origin and a trailing `:80` on an
   `http` origin. Do not drop other ports.
5. The result contains no trailing query separator and no empty `?`.

The comparison with the `htu` claim is an exact string comparison after both
sides have been normalized with the steps above. A proof whose `htu` was
minted with a query string is therefore still acceptable when the same
resource is requested with a query string of a different shape, and a proof
minted for a different path is never acceptable. Prefix matches are not
acceptable: `/a` does not cover `/ab`.

## 5. `ath` computation

The access token is the exact byte string that follows the `DPoP ` scheme in
the `Authorization` header, with no trimming. `ath` is the base64url encoding
(no padding) of the SHA-256 digest of those bytes. Changing a single byte of
the token changes `ath` with overwhelming probability, so a proof cannot be
lifted onto another token.

## 6. Replay

A proof is unique per `(jkt, jti)`, where `jkt` is the RFC 7638 fingerprint of
the proof header key. The resource server records each accepted tuple for at
least the remaining lifetime of the proof (`exp`, or `iat + 60` when that is
later than `exp` is not considered; the earlier of the proof's own validity
bounds governs). After that, the same tuple is rejected as a replay.

Two clients that happen to generate the same `jti` are distinct because their
keys, and hence their `jkt`, differ. A cache keyed only by `jti` would let one
client evict or block the other and is not conformant. Concurrent arrivals of
the same proof must not both be accepted; the replay check must be atomic with
respect to itself.

Replay rejection uses the same `401` and `WWW-Authenticate: DPoP
error=invalid_dpop_proof` as any other invalid proof.

## 7. Fingerprints (RFC 7638)

The fingerprint of a proof key is the base64url (no padding) encoding of the
SHA-256 digest of the key's canonical JSON representation.

The canonical representation contains exactly the required members for the key
type, in lexicographic order, with no whitespace:

- EC (`kty: "EC"`): `{"crv":...,"kty":"EC","x":...,"y":...}`
- RSA (`kty: "RSA"`): `{"e":...,"kty":"RSA","n":...}`

Members such as `kid`, `alg`, `use` and `key_ops` are not part of the
fingerprint, even when present in the proof header, and are omitted from the
canonical form. The member values are the base64url strings exactly as they
appear; they are not re-encoded and not normalized.

Reference values for the examples in the RFC:

```
EC  {"crv":"P-256",
     "kty":"EC",
     "x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU",
     "y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"}
 -> oKIywvGUpTVTyxMQ3bwIIeQUudfr_CkLMjCE19ECD-U

RSA {"e":"AQAB",
     "kty":"RSA",
     "n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbf
          AAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMs
          tn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQ
          R0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5h
          ajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRw
          r3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"}
 -> NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs
```

## 8. Token binding

The access token's introspection response carries `cnf.jkt`. That value MUST
equal the RFC 7638 fingerprint (§7) of the key in the proof header. The
comparison is exact: base64url is case-sensitive and values are compared as
opaque strings without case folding or re-encoding.

A token with no `cnf.jkt`, or a proof whose key type is not supported for
fingerprinting, results in rejection. Such a token or proof is never treated
as "unbound but acceptable".
