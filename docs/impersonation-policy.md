# Impersonation policy

This document defines the normative behaviour of actrail. It describes *what*
must hold, not how the implementation is arranged.

Terminology:

- **principal** — an identity known to IAM. A principal is either a `user` or a
  `service_account`.
- **actor** — the principal that asks to act as somebody else.
- **target** — the principal whose identity the resulting token speaks for.
- **context** — request attributes supplied by the caller, such as `tenant`,
  `time_window`, `resource_label` and `source_network`.
- **binding** — an entry in `policy/bindings.json` with `members`, `role`,
  `target` and `conditions`.

## 1. Bindings

A binding authorises every principal in `members` to impersonate `target`
under `role`. A binding is only usable when the request context satisfies all
of its conditions.

Bindings are matched by `(actor, target)`. A role name is descriptive; it does
not grant scopes or permissions by itself.

## 2. Conditions

Every key present in a binding's `conditions` object must be evaluated against
the request context.

1. A condition holds only when the corresponding context attribute exists and
   matches the expected value.
2. All conditions on a binding must hold. A single failing condition makes that
   binding inapplicable.
3. A condition key that the implementation does not understand makes the
   binding inapplicable. Silently ignoring an unknown key is not permitted.
   Rejecting such a document while loading is also acceptable.
4. A condition whose key is absent from the context does not hold.
5. When several bindings match `(actor, target)`, the request is authorised if
   at least one applicable binding exists.

`time_window` is an object with `start`, `end`, `days` and `timezone`. The
request must occur inside the window on one of the listed days. `resource_label`
is an object of label names to expected values, all of which must be present on
the target resource context. `source_network` is a list of CIDR blocks; the
request's source address must fall inside one of them.

## 3. Scopes

A token carries the scopes actually granted to it. Scopes are additive
capabilities, so a delegation must never widen them.

For an impersonation request the granted scopes are the intersection of:

1. the scopes the caller currently holds (on its own token, or its principal
   record when it presents no token),
2. the scopes held by the target principal, and
3. the scopes explicitly requested.

A request for a scope outside that intersection is rejected; the token is not
issued. Scopes are never combined as a union. The resulting token's `scope`
claim contains exactly the granted set, with no duplicates.

## 4. Lifetime

A token always has a positive, bounded lifetime.

1. The lifetime is the minimum of the requested TTL, the caller's remaining
   token lifetime, and the target principal's `max_session_seconds`.
2. A missing TTL is rejected.
3. A negative or zero TTL is rejected.
4. The issued `exp` is strictly greater than `iat`, and `exp` is always present
   and non-zero.
5. A token whose `exp` is absent, zero, or in the past is not valid.

Clamping a request to the smallest permitted lifetime is preferred over
issuing a token that outlives its source; in either case the issued lifetime
must not exceed any of the three limits.

## 5. Delegation chains

A token records the path by which the current holder obtained the identity.

1. The chain contains every principal that delegated, in origin-first order,
   and ends with the current subject (`sub`).
2. A chain may contain at most three delegation hops. Four or more hops are
   rejected.
3. Every hop is authorised on its own terms. The hop from principal `P` to
   principal `Q` must be checked against the binding for `(P, Q)`. Checking
   every hop against the origin of the chain is not sufficient.
4. A principal may not appear twice in a chain. In particular `A -> B -> A` is
   rejected, and a chain may not be extended indefinitely by alternating
   between two principals.
5. Extending a chain requires presenting the token that represents the current
   holder; the `sub` of that token must equal the actor requesting the next
   hop.

## 6. Downstream decisions

A downstream service asks whether a token permits an action on a resource.

1. The decision is made for `sub`, the principal the token speaks for.
2. The permissions of `act.sub`, and of any earlier delegator, do not grant
   anything to the token. Delegation transfers authority only within what the
   target itself may do.
3. The actor recorded in `act` is used for audit and for the no-escalation
   rule: a delegated call may never exercise a permission that the target does
   not hold, regardless of what the actor holds.
4. The token's `scope` claim limits the actions available to the subject; an
   action that needs a scope absent from the token is denied.
5. A malformed, expired, or incorrectly signed token is denied.

## 7. Audit

Every decision produces one audit record.

1. The record identifies `sub`, the action, the resource, the decision and the
   time.
2. The record carries the complete `act` chain, in origin-first order, so that
   the principal that started the delegation can be identified. Recording only
   the immediate delegator is not sufficient.
3. Denied decisions are recorded as well as allowed ones.
