# Token format

actrail issues compact, HMAC-signed tokens. The envelope follows the familiar
JWT shape so that existing gateway tooling can parse the framing, but only the
algorithm below is supported.

## 1. Envelope

A token is three base64url segments joined by dots, with padding removed:

```
<header>.<payload>.<signature>
```

The signature is the HMAC-SHA256 of the ASCII string
`"<header>.<payload>"` under the actrail signing key, base64url-encoded the same
way. The signing key comes from the `ACTRAIL_HMAC_KEY` environment variable or
from an explicit argument; the repository ships a development fallback for
local runs only. Verification must compare signatures in constant time and
must reject tokens whose header advertises an algorithm other than `HS256`.

The header is `{"alg": "HS256", "typ": "JWT", "iss": "actrail"}`.

## 2. Payload

| claim | type | meaning |
|---|---|---|
| `iss` | string | always `actrail` |
| `sub` | string | principal the token speaks for (the impersonated target) |
| `scope` | string | space-separated granted scopes |
| `iat` | integer | issue time, POSIX seconds |
| `exp` | integer | expiry time, POSIX seconds; always present and greater than `iat` |
| `jti` | string | unique token id |
| `delegated_by` | string | principal that requested this hop |
| `act` | object | delegation chain, see below |

### `act`

`act` nests one object per delegator. The outermost object is the principal
that performed the most recent delegation; the innermost is the principal that
started the chain.

```json
{
  "sub": "svc-report",
  "act": {
    "sub": "bob",
    "act": {"sub": "alice"}
  }
}
```

Read origin-first, this is `alice -> bob -> svc-report`. A token minted by
`alice` directly for itself carries no `act` claim.

Because the chain is read origin-first throughout the system, `act` must be
reconstructed from the outer object inwards; the innermost `sub` is the origin
of the delegation.

## 3. Validity

A token is usable when:

- the signature verifies under the current key,
- `iss` is `actrail`,
- `sub` is present and non-empty,
- `exp` is present, non-zero, and greater than the current time,
- `iat` is not in the future.

A token with `exp` equal to `0`, missing, or otherwise implying "no expiry" is
invalid. Consumers must treat such a token as rejected rather than as
unbounded.

## 4. Scopes

`scope` holds exactly the granted scopes, space-separated and duplicate-free.
Issuance rules are in `docs/impersonation-policy.md` §3. A downstream service
that needs a particular scope checks membership in this claim before
consulting permissions.

## 5. Chain length

`act` may nest at most two levels (three hops including the subject). A token
whose `act` chain names four or five principals is invalid regardless of its
signature, because a valid signature only proves that some actrail component
signed it, not that the chain was authorised.

## 6. Compatibility

Unknown claims are ignored by parsers but never by policy evaluation. Any new
claim that changes authorisation must be introduced with a corresponding
policy rule; a token claim that the policy layer does not understand cannot
widen a decision.
