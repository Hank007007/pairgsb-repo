# Design notes

This note describes the intended behaviour of actrail and the responsibility of
each module. It states rules and invariants only; the normative wording lives in
`docs/impersonation-policy.md`, `docs/token-format.md` and
`docs/threat-model.md`.

## Model

Three principal roles appear in every flow:

- the **originator** — the principal that starts a delegation chain,
- a **holder** — the principal currently presenting a token, which may itself
  be an impersonated identity,
- the **subject** — the principal named by `sub`, whose identity the token
  speaks for.

A token always answers two separate questions:

1. *Whose identity is this?* — `sub`.
2. *How did the holder obtain it?* — the `act` chain, origin first.

Keeping those questions separate is the central invariant. Authorisation looks
only at the first; audit and anti-escalation look at the second.

## Module responsibilities

### `actrail/policy.py`

Owns the policy documents and the question "may `actor` act as `target` under
this context?". It loads principals and bindings, rejects documents that
reference unknown principals, and evaluates the conditions attached to a
binding.

Invariants:

- A binding is a conjunction: every condition must hold.
- A condition that cannot be evaluated — an unknown key, a missing context
  attribute, a value of the wrong shape — does not hold. It must not be treated
  as satisfied.
- `can_impersonate` is a pure question about the pair `(actor, target)` and the
  supplied context. It does not consider chains.

The set of condition keys understood by the evaluator is part of the policy
contract. Adding a key to a binding without teaching the evaluator about it
must make the binding unusable, not broader.

### `actrail/token.py`

Owns issuance and parsing.

Issuance has three independent limits:

- **scopes** — the granted set is bounded by both the caller's inventory and
  the target's inventory, and by the request. Each of the three contributes an
  upper bound; the result is their intersection.
- **lifetime** — the issued lifetime is bounded by the request, by how long the
  caller's own credential remains valid, and by the target's session limit.
  There is no configuration in which a token is issued without a finite
  positive lifetime.
- **chain** — extending a chain consumes one hop. Each new link is authorised
  against the holder that performs it, not against the originator. Repeated
  principals and over-long chains are refused before a token is produced.

Parsed claims are validated for signature, issuer and time window. A claim
that would mean "no expiry" is not a valid claim.

### `actrail/authorize.py`

Owns the downstream question "may this token perform `action` on `resource`?".

The answer is a function of `sub` and `sub` alone, further limited by the scope
claim. The `act` chain may reduce what is allowed — a delegated call cannot
exceed what the subject itself may do — but it never adds permissions. A
delegator's own grants are irrelevant to the decision except as a ceiling
already implied by the subject's grants.

### `actrail/audit.py`

Owns attribution. One record per decision, containing the subject, the action,
the resource, the outcome and the full `act` chain in origin-first order. An
investigator must be able to read the originator from the record without
consulting any other store.

### `actrail/clock.py`

A single injectable source of time. Token lifetime, expiry checks, time windows
and audit timestamps all read from the same clock so that a test can move time
forward and observe every component consistently.

### `actrail/errors.py`

A shallow hierarchy so that callers can distinguish policy refusals, token
failures and chain failures without matching on message text.

## Flow

```
can_impersonate(actor, target, ctx)
        │  policy only; no token involved
        ▼
mint_impersonated(actor, target, scopes, ttl, actor_token=...)
        │  1. resolve the holder's chain from actor_token
        │  2. verify the holder really is `actor`
        │  3. authorise this link against the holder
        │  4. bound scopes and lifetime
        │  5. append the link and sign
        ▼
decide(token, action, resource)
        │  1. verify token
        │  2. decide for sub
        │  3. audit sub + full chain + outcome
        ▼
     allow / deny
```

## Failure posture

Every unexpected shape is a refusal. In particular: missing context attributes,
unknown condition keys, missing or non-positive TTLs, scopes outside either
inventory, repeated principals, over-long chains, absent or zero `exp`, and
tokens whose subject is not a known principal. Failing closed keeps the
authorisation surface small; failing open quietly is how the escalation paths
in `docs/threat-model.md` become reachable.
