# Threat model

This document enumerates the privilege-escalation paths that have been
considered for actrail. Each entry states the attack and the rule that governs
it. Entries marked **待补** describe behaviour the specification requires but
that is not covered end to end yet; they need to be closed before the service is
considered production-ready.

## T1 — Stolen operator token

An attacker obtains a short-lived impersonation token for a service account.
Mitigation: short lifetimes, audience-scoped scopes and full audit of every
decision. The `act` chain names the delegator, so misuse is attributable.

## T2 — Forged or tampered token

An attacker edits `sub`, `scope` or `exp` and re-signs with a guessed key.
Mitigation: HMAC-SHA256 over the encoded segments, constant-time comparison and
a key kept outside the repository.

## T3 — Chained delegation bypassing binding checks

`A` may impersonate `B`, and `B` may impersonate `C`. When the chain is
extended, each link must be authorised on its own terms: `A -> B` needs a
binding for `(A, B)` and `B -> C` needs a binding for `(B, C)`. A check that
falls back to the originator of the chain lets a delegate reach accounts it has
no authority over.

The same entry covers depth and cycles. A chain longer than three hops, or a
chain that repeats a principal such as `A -> B -> A`, must be rejected. A cycle
otherwise lets two principals renew each other's tokens without bound.

Status: **待补**.

## T4 — Scope union instead of intersection

A caller that holds `documents:read` asks for `documents:write` on a target
that holds both. The issued scope set must be the intersection of the caller's
scopes, the target's scopes and the request; a request outside that intersection
is refused. Combining the two inventories as a union hands the caller a
capability it never had.

The related lifetime rule belongs here as well. A token must not outlive the
caller's own token or the target's `max_session_seconds`, and a missing or
negative TTL must be refused rather than turned into a non-expiring token.

Status: **待补**.

## T5 — Acting identity confusion downstream

A downstream service evaluates `act.sub` instead of `sub`, or evaluates both and
accepts either. A token issued for a low-privilege service account then carries
the permissions of whoever impersonated it. The decision is made for `sub`;
`act` is metadata plus a ceiling that forbids exceeding the target's own
permissions.

## T6 — Condition skipping

A binding carrying tenant, time or network restrictions is treated as
unconditional, so an attacker matches it from another tenant or outside the
allowed window. Unknown keys make a binding inapplicable, and a condition whose
attribute is absent from the request context does not hold.

## T7 — Audit truncation

Only the immediate delegator is recorded. After two delegations the origin of
the call is lost, and an incident review cannot establish who started it. The
audit record carries the full chain.

## T8 — Token replay across services

Tokens are bearer credentials. Mitigation: short TTLs, `jti` for correlation and
service-side rate limiting. Revocation lists are out of scope for this
repository.

## Out of scope

- Compromise of the signing key itself.
- Network-level attacks between actrail and downstream services.
- Storage and retention of the audit log; only record contents are specified.
