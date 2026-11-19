# tokengrit design

This document states the behaviour the library is required to have. It is a
contract, not a guide to the current implementation.

Related specifications: `docs/token-format.md` (grammar and public outcomes),
`docs/hashing.md` (storage and comparison of secrets).

## §1 Scope

tokengrit manages API tokens for internal services. A token has four
lifecycle operations:

* **issue** — mint a new token and return its plaintext exactly once;
* **verify** — decide whether a presented value may be used for a request;
* **rotate** — replace an existing token with a successor while giving the
  predecessor a bounded overlap window;
* **revoke** — permanently disable a token.

The store is SQLite via the standard library. No network services, no
third-party dependencies.

## §2 Issuance

* A successful issuance returns the formatted token, its identifier, its
  family identifier, its scopes, and its timestamps. This return value is the
  only time the secret is available; neither the library nor its audit trail
  retains it.
* The identifier must be unique among live tokens at the moment of issuance.
  Random allocation alone is not sufficient: the store must reject or retry
  an identifier that is already present. Two live records with the same
  identifier must never be created by the issue path.
* A freshly issued token is immediately usable and carries exactly the scopes
  requested. An empty scope set is valid and means "no scope-gated
  operations".
* If an expiry is supplied, the token stops verifying at that instant.
  Otherwise it does not expire on its own.
* Every successful issuance appends one audit event (§7).

## §3 Verification

Verification accepts the raw token and an optional required scope. It reports
one of the outcomes in `docs/token-format.md` §3. In order:

1. Values that are not well-formed are rejected with `invalid_token`.
2. The identifier is resolved to candidate records (§4).
3. The presented secret is compared against each candidate's stored hash
   using the rules in `docs/hashing.md` §3. No candidate matching means
   `invalid_token`.
4. Revoked credentials yield `revoked`.
5. Credentials past their expiry, or past the end of a rotation grace window,
   yield `expired`.
6. A missing required scope yields `insufficient_scope`.

Steps 4–6 apply only to a token whose secret actually matched: the
authentication decision comes first, so lifecycle state is never revealed to
a caller who does not hold valid key material.

The public result for all authentication failures is identical in every
observable field (`docs/token-format.md` §3). Failure events in the audit
trail retain the internal category (`docs/token-format.md` §4).

## §4 Identifier resolution

An identifier can match more than one record (for example after a collision
that was created by an older release). Resolution therefore returns **all**
records with that identifier, and verification succeeds if any one record's
hash matches the presented secret. The matched record — not merely the first
row with that identifier — supplies the family, scopes, and lifecycle state
used by the rest of verification.

## §5 Rotation

* The caller presents a currently usable token. An unusable or invalid
  predecessor produces no new token and reports a failure outcome; it does
  not disclose anything beyond `docs/token-format.md` §3.
* The successor is a new record in the **same family**, with a new identifier
  and a new secret. It inherits the predecessor's scopes unless the caller
  supplies a replacement scope set explicitly.
* Unless an explicit expiry is supplied, the successor keeps the
  predecessor's expiry policy.
* The predecessor enters a grace window: it continues to verify for the
  duration of that window. **The window starts at the moment of rotation**,
  not at the predecessor's creation time. The default duration is 24 hours.
* When an expiry is known for the predecessor, the window ends no later than
  that expiry.
* Once the window has elapsed the predecessor yields `expired`. Rotation does
  not un-revoke: a revoked predecessor cannot be rotated.
* Every successful rotation appends one audit event (§7) naming the old and
  new identifiers.

## §6 Revocation

* Revocation applies to the entire **family** identified by the token's
  `family_id`. Revoking any one token makes every token in the family
  unusable immediately, including the successor and predecessor of a
  rotation and any token issued independently with that family identifier.
* Revocation is idempotent. Revoking an already-revoked token changes no
  state and is not an error.
* Revoked tokens remain revoked; a later rotation cannot revive them.
* Attempting to revoke a value that is not a well-formed token (or a known
  identifier) is a no-op that reports zero affected tokens.
* A successful revocation appends one audit event per operation.

## §7 Audit trail

* Events are append-only and ordered by insertion. Each event has a
  timestamp, an event name, the relevant token identifier, the family
  identifier, and a structured payload.
* The payload may contain the identifier and the public prefix, scopes,
  outcome/reason, and internal failure category.
* The payload must **not** contain the token's secret, the full plaintext
  token, the stored hash, or any portion of the secret. The identifier is not
  secret and is expected to be present.
* Failure events distinguish `malformed`, `unknown_prefix`, and `bad_secret`
  internally, even though callers cannot (§3).
* Audit writes are part of the operation: they are committed with it. No
  event is written for an operation that was rejected before it took effect
  (for example a rotation whose predecessor fails verification). A
  revocation attempt that changes no rows is still recorded, with zero
  affected tokens.

## §8 Time

Lifecycle decisions take the current time from a single clock reading per
operation. All comparisons use that value consistently, so grace and expiry
boundaries are deterministic. Callers and tests may supply the time
explicitly; when they do, it replaces the clock reading everywhere in that
call.

## §9 Storage

The schema is fixed for this release; migration is out of scope. Records are
append-oriented: rotation adds a row and marks the predecessor, revocation
updates rows in place. The store is safe for the single-process use this
library targets; concurrent writers are not part of this release.
