# One-time tokens — behavioural contract

This document states what a one-time token must do. It is normative: any
implementation in this repository is expected to satisfy every clause. How the
clauses are realised is left to the implementation.

Terminology:

- **token** — the opaque secret string that is handed to the end user. It is
  never persisted and never logged.
- **record** — the server-side data associated with a token: an internal id,
  the bound `subject`, the bound `purpose`, arbitrary `meta`, an issue time, an
  expiry time and a digest (§2).
- **pepper** — a server-side secret that is not part of the repository.

## §1 Issuing

1. `issue()` accepts a `subject`, a `purpose` and optional `meta`.
2. The `subject` must be a non-empty string once trimmed. Empty or blank
   subjects are rejected before anything is written.
3. The `purpose` must be one of the purposes the service is configured with
   (by default `password-reset`, `invite`, `api-key-handoff`). Unknown purposes
   are rejected before anything is written.
4. The token is generated from a cryptographically secure random source and
   must be unique in practice.
5. The plaintext token is returned to the caller exactly once, in the result of
   `issue()`. It must not appear in the record, in storage, in the audit log or
   in any error message.
6. A record is written with an expiry time equal to the issue time plus the
   requested lifetime. Lifetimes must be positive and must not exceed the
   configured maximum.

## §2 Digests and pepper

1. The record holds a **digest** of the token, never the token itself.
2. The digest is computed with a keyed operation that mixes in the service
   pepper: two records for the same token under two different peppers must have
   different digests.
3. Changing the pepper invalidates every previously issued token: a token
   issued under pepper A must not be accepted while the service is configured
   with pepper B, even if the record is still present in storage.
4. Comparing a freshly computed digest with a stored digest must not leak
   information through comparison timing; the comparison must take the same
   amount of work regardless of where the two values first differ.
5. The digest is stable for a fixed (token, pepper) pair, including across
   process restarts and across the memory and file stores.
6. No layer of the system may retain the plaintext token. That includes the
   in-memory index used for lookups, the JSON file written to disk, temporary
   files created while writing it, and the audit log.

## §3 Binding to subject and purpose

1. A record is bound to the `subject` and the `purpose` that were supplied at
   issue time.
2. Both bindings are enforced when the token is consumed. Redeeming a token
   with a different `subject` than the one it was issued for must fail, even if
   the record exists and the token itself is genuine.
3. Redeeming a token with a different `purpose` than the one it was issued for
   must fail in the same way.
4. A failed binding check must not consume or remove the record: the legitimate
   holder must still be able to redeem the token with the correct subject and
   purpose.
5. The subject and purpose returned by a successful consume are those from the
   record, not those supplied by the caller.

## §4 One-time use and atomicity

1. A token may be redeemed at most once. The second and every later redemption
   attempt must fail with an "already used" outcome.
2. Redemption is atomic with respect to concurrent callers. If N callers
   present the same token at the same time, exactly one succeeds and the other
   N-1 receive the "already used" outcome. Partial success is not acceptable.
3. Redemption must not report success unless the record was actually removed
   from storage. If storage fails while removing the record, the caller must
   receive an error and must not receive the payload.
4. An unknown token is reported as unknown. A token that was valid and has
   already been redeemed is reported as already used. The two outcomes must be
   distinguishable.
5. A record that has been consumed must not be redeemable again, even if the
   same token string is issued again later by an independent issue call (the
   new issue creates a new record and its own one-time redemption).

## §5 Expiry

1. Expiry is evaluated against the service clock, not against a clock supplied
   by the caller of `consume()` and not against wall-clock time read from the
   storage layer.
2. A token whose expiry time is at or before the current service time must not
   be redeemable. Attempting to redeem it fails with an "expired" outcome that
   is distinguishable from "unknown" and from "already used".
3. Expired records must not linger in storage. When the service observes that a
   record is expired it is removed, and the storage layer exposes a pruning
   operation that removes every record whose expiry has passed.
4. Pruning must be idempotent: running it twice, or running it on a store with
   no expired records, is harmless and reports how many records were removed.

## §6 Audit log

1. Every successful issue, every successful consume and every rejected
   redemption attempt is recorded as a discrete event with a timestamp taken
   from the service clock.
2. Each event carries the internal record id where one is known, and the
   subject and purpose associated with the operation.
3. The audit log must not contain the token, any prefix of the token, any
   suffix of the token, or any other value from which the token can be
   recovered. This applies regardless of token length: a token shorter than a
   typical fingerprint window must not be logged in full.
4. If a non-sensitive correlation value is useful for support purposes it may
   be recorded, but it must be derived so that it cannot be turned back into the
   token and so that it does not expose any contiguous part of the token.
5. Error messages and thrown errors follow the same rule: they never include
   the token or a fragment of it.

## §7 Outcomes

A redemption attempt produces exactly one of the following outcomes:

| Outcome | Meaning |
|---|---|
| success | The record was removed and the payload is returned. |
| unknown | No record corresponds to the presented token. |
| already used | The token was previously redeemed and its record is gone. |
| expired | The record exists but its expiry has passed; it is removed. |
| subject mismatch | The token is genuine but bound to a different subject. |
| purpose mismatch | The token is genuine but bound to a different purpose. |
| storage failure | The record could not be removed; nothing was redeemed. |

Outcomes must be distinguishable by the caller. An implementation may encode
them as distinct error codes.
