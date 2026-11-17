# Delegation chains — normative design

This document is the specification for `capbaton`. It states what a
conforming implementation must do. It deliberately describes behaviour
only; it does not prescribe internal structure.

A **delegation chain** is an ordered slice of links, from the root link
(the on-call engineer's own capability token) to the leaf link (the
credential presented by the downstream caller). Each link after the
first was derived from the one before it by *attenuation*: the holder of
a link issues a strictly weaker link to somebody else.

A **scope** is a string of the form `action:resource`, for example
`s3:read:bucket/incident-42`. The action is everything before the final
colon; the resource is everything after it and is a `/`-separated path.

A **caveat** is a named restriction carried by a link. `expires` is the
caveat whose value is an RFC 3339 timestamp; a link carrying it must not
be honoured at or after that instant. Caveats other than `expires` are
opaque to this library and are enforced by the resource server.

## §1 Structure of a link

A link carries a payload (token id, issuer, subject, scopes, caveats,
the MAC of the link it was derived from, and its issue time) together
with a MAC over that payload. The root link has no parent, and its
parent MAC is empty. Every other link records the MAC of its immediate
parent.

`[MUST]` A chain presented for verification is ordered root-first and
leaf-last, is non-empty, and contains no gaps.

## §2 Integrity of each link

`[MUST]` The MAC of a link authenticates the link's entire payload: every
field of the declaration, and the parent MAC it records. Changing any
byte of any field invalidates the MAC. The MAC is computed under the
key of the link's issuer; a link whose issuer has no known key is
rejected.

## §3 Integrity of the chain as a whole

`[MUST]` Verification establishes that the presented links form a single
unbroken derivation. For each link after the first, the parent MAC it
records must equal the MAC of the link immediately preceding it in the
slice. A link that was issued into a different chain therefore cannot be
substituted into this one, even though each link's own MAC is valid.
`[MUST]` A link that appears out of order, is duplicated, or is omitted
is rejected.

## §4 No escalation on attenuation

`[MUST]` Attenuation produces a link whose scopes are a proper subset of
the parent's scopes. Scope containment is decided per scope pair:

- actions must be equal as strings; and
- the child resource must be contained in the parent resource when the
  parent's path segments are a leading run of the child's segments.

`bucket/ab` and `bucket/abc` are siblings: neither contains the other.
`bucket/a` contains `bucket/a/logs` but not `bucket/ab`. A resource
contains itself, so repeated attenuation of the same resource is legal
only when the action set shrinks.

`[MUST]` Attenuation fails, returning an error and producing no link,
when any requested scope is outside the parent's scopes, when the
requested scope set is identical to the parent's, or when the claimed
issuer is not the parent's subject.

## §5 Caveats are inherited, never dropped

`[MUST]` A child link carries every caveat of its parent, unchanged. It
may add caveats. Attenuation fails, producing no link, if any parent
caveat is absent from the child's caveat list, or if a caveat of the
same name is present with a different value. In particular, a holder
cannot shed an `expires` restriction by attenuating.

`[MUST]` During verification, the effective caveat set of a chain is the
union of the caveats of all of its links. A chain whose effective
`expires` has passed is rejected, no matter which link contributed the
caveat. A malformed `expires` value invalidates the chain.

## §6 Revocation

`[MUST]` A token identifier revokes the link that carries it and every
link derived from it. Verification rejects a chain when the token id of
*any* link in the chain is on the revocation list — revoking an
intermediate link invalidates all of its descendants.

## §7 Length bound

`[MUST]` A chain of more than six links is rejected before any other
work is done. Six links are accepted (subject to the checks above).

## §8 Time

`[MUST]` Expiry is evaluated against a supplied clock, so that verifiers
are testable and consistent. A link is expired when the current time is
at or after its effective expiry.

## §9 Errors

`[MUST]` Failures are reported as errors that callers can distinguish
with `errors.Is`. Attenuation failures produce no link. Verification
failures produce no partial result and no side effects.

## Verifier flow (informative)

1. Reject empty and over-long chains (§7).
2. Resolve keys for every issuer and authenticate every link (§2).
3. Confirm the parent-MAC links form one chain (§3).
4. Evaluate the union of caveats, including expiry (§5, §8).
5. Consult the revocation list for every link (§6).
