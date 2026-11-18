# Capability chains — design and conformance

This document states the behaviour `caveatchain` is required to have. It is
normative: it describes *what* must hold, not how to implement it. Anything
not written here is an implementation detail.

A capability token is a bearer credential. The issuer signs a root; anybody
holding the token may append caveats that only narrow it; a verifier that was
given the root key can validate the token without contacting the issuer.
The token is a JSON object (see `caveatchain/token.py` for the wire form):
a root id, an ordered caveat list, and one signature per level.

## §1 Signature coverage

1. The root signature commits to the root identifier and to the domain
   separation label; two tokens with different root ids must never share a
   root signature under the same key.
2. The signature at level *i* (for *i* ≥ 1) must commit to the root
   signature and to the ordered caveat list up to and including the *i*-th
   caveat.
3. The tail signature is the last signature in the list. It is the value a
   verifier ultimately relies on.

## §2 Caveat framing (unambiguous binding)

1. The signature input for a caveat list must be an injective function of the
   *list* — an injective function of both the number of caveats and the bytes
   of each caveat, in order.
2. Concretely: for two different sequences of caveat payloads, the resulting
   signature must differ. In particular `["ab", "c"]` and `["a", "bc"]` are
   different chains and must produce different signatures.
3. Concatenating payloads without a separator or a length prefix does not
   satisfy §2.1. Any framing that is unambiguous suffices; the framing is an
   internal detail and need not be interoperable with any other
   implementation.
4. Changing a single byte of any caveat changes that level's signature and
   every later level's signature.

## §3 Verification order

1. A verifier must reject a token whose signature chain does not validate,
   regardless of whether its caveats are satisfied. A token with a broken
   signature is not a valid token, even if every caveat accepts the request.
2. Signature validation and caveat evaluation are independent checks; both
   must pass. Neither one may be used to excuse the other.
3. A verifier should check the structural integrity of the token first
   (decode, signature count matches caveat count + 1), then the signature
   chain, then caveats, then revocation. The observable requirement is that
   *no* accepting path exists when any one of these fails.
4. A malformed token — unparseable, wrong version, missing root id, wrong
   number of signatures — is rejected rather than repaired.

## §4 Chain validation is all-or-nothing

1. Every level's signature must match. A mismatch at level *i* invalidates
   the entire token.
2. Skipping an unmatched level and continuing with later levels is not
   permitted. The tail signature alone is not sufficient evidence that the
   earlier levels were correct, because level *i*'s computation depends on
   the ordering and content of earlier caveats, but the *stored* earlier
   signatures are part of what the verifier must check: a token whose level
   *i* signature was replaced must be rejected even if levels *i+1..n* were
   recomputed correctly over the genuine earlier payloads. (If the attacker
   can recompute later levels they also have the key, in which case the token
   is legitimate; the requirement is about detecting tampering, not about
   defending against a key holder.)
3. The comparison must be constant-time with respect to the signature bytes.

## §5 Time caveats

1. An expiry caveat carries an expiry instant. The token is valid while the
   current time has not passed it.
2. Validity is decided against a single clock reading taken once per
   verification of the whole chain. Two caveats in one chain must not observe
   different clock readings, and two verifications of the same token at the
   same instant must agree.
3. The comparison is exact at the boundary: at exactly the expiry instant the
   token is expired. A token whose expiry is `T` must fail at `T` and at
   `T + 1`, and must pass at `T - 1`.
4. Expiry instants are integral seconds. Sub-second values must not change the
   answer, and a system clock that reports fractional seconds must not cause
   the same token to alternate between accepted and rejected.
5. Callers supply the verification context; the context's clock reading is
   what §5.2 and §5.3 refer to. A verifier must not read the clock again
   while answering a single request.

## §6 Revocation

1. Revocation is keyed by root identifier, not by caveat content. A root id
   is registered in the `KeyStore` and is the identity that a revocation list
   refers to.
2. The root id of a token is the `root_id` field at the head of the token
   (equivalently, the root signature's input). It is never taken from the body
   of a caveat: caveats are attacker-supplied data.
3. A nested caveat embeds a complete token. If the inner token's root is
   revoked, the chain containing the nested caveat must not verify, and vice
   versa: revoking the outer root does not revoke an independently issued
   inner token, but the outer chain is still rejected. In short, *every* root
   referenced by the chain — the head root and the root of every embedded
   token, recursively — is looked up.
4. A revocation entry has its own lifetime. An entry whose lifetime has
   elapsed no longer revokes anything; pruning such entries must not change
   any verification answer.
5. The revocation list's clock is the list's own clock, not the caller's
   context clock; sharing one list between callers must be safe.

## §7 Path caveats

1. A path caveat grants a subtree rooted at its prefix. The prefix and the
   candidate path are normalized before comparison: both are absolute paths
   with `.` and `..` segments resolved lexically, repeated slashes collapsed,
   and no trailing slash except for the root `/`.
2. Containment is by path segment, not by string prefix. `/docs/secret` and
   `/docs` are inside a `/docs` subtree; `/docs-internal` is not.
3. Any candidate path whose normalization escapes above `/` (for example
   `/docs/../secret`) is outside every non-root subtree and must be rejected.
4. A trailing slash on either side is insignificant: `/docs/` and `/docs`
   denote the same subtree.
5. An empty or relative candidate path is rejected.

## §8 Tenant caveats

1. A tenant caveat accepts exactly the request whose tenant is string-equal
   to the caveat's tenant.
2. A missing request tenant is never equal to a caveat tenant, including the
   empty string.
3. Tenant comparison is exact; no case folding, trimming, or prefix matching.

## §9 Nesting

1. A nested caveat is satisfied only if the embedded token verifies under the
   same rules as a top-level token, including revocation (§6.3) and caveats.
2. Nesting may recurse; implementations must bound the depth and reject a
   chain that exceeds it rather than recursing without limit.
3. A nested caveat with a missing or malformed embedded token is not
   satisfied.

## §10 Attenuation

1. Appending a caveat never widens the token: the new chain's satisfied
   request set is a subset of the old chain's.
2. Appending a caveat invalidates all previous signatures at and after the
   appended position; a verifier must never accept a token whose level
   signatures do not match the caveat list it carries (§4.1).
3. The wire form is versioned. A version that the verifier does not
   understand is rejected, not interpreted as version 1.
