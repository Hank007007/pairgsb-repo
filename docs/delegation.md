# Delegation

Delegation is the mode in which a service acts on behalf of a subject, rather
than speaking purely as the subject. It is signalled by the presence of an
`actor_token` (or by `mode=delegation`).

## 1. The `act` claim

An access token issued through delegation carries an `act` claim that records
the acting party. `act` is a nested object:

```json
{"act": {"sub": "svc-front", "act": {"sub": "svc-gateway"}}}
```

Each level carries the full information for that level, and the chain is
preserved exactly as it was presented, apart from the new outermost level. The
new actor is added as a new outermost level; existing levels keep their order
and their nesting.

The chain may contain at most three levels. A chain of three levels is
accepted and written out unchanged.

## 2. Depth limit

If the resulting chain would exceed three levels, the exchange fails with
`access_denied`. The chain is never truncated, flattened, or replaced by a
depth counter.

## 3. Downscoping and refresh

An exchange may issue a token with less scope than the subject token holds
(for example, a delegation that only needs read access). The refresh grant
created by that exchange records the reduced scope, not the subject's original
scope.

Redeeming that refresh grant yields a token with the reduced scope. Refresh
never restores scope that the exchange removed. When a user's permissions are
reduced, subsequent refreshes reflect the reduction; they do not recover the
older, broader scope.

## 4. Impersonation

Impersonation (no `actor_token`) produces a token that speaks as the subject.
It carries no `act` claim. Supplying `actor_token` together with
`mode=impersonation` is a malformed request.
