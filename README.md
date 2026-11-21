# actrail

actrail is the impersonation service for internal IAM. A principal may obtain a
short-lived token that speaks for another principal — usually a service
account — and call internal APIs as that identity. Every token records the
chain of principals that delegated it in an `act` claim.

## Public API

```python
from actrail import can_impersonate, mint_impersonated, decide

can_impersonate("alice", "svc-report", {"tenant": "finance"})     # -> bool

token = mint_impersonated("alice", "svc-report", ["documents:read"], 600)

decide(token, "documents:read", "report/q3")                      # -> bool
```

All three accept keyword arguments for the loaded policy, an injectable clock
and the HMAC key. `mint_impersonated` additionally accepts `actor_token` when
the request arrives on an already impersonated token, which extends the
delegation chain by one hop.

The package has no third-party dependencies.

## Layout

| path | contents |
|---|---|
| `actrail/policy.py` | binding and principal loading, condition evaluation, `can_impersonate` |
| `actrail/token.py` | token envelope, signing, minting, chain helpers |
| `actrail/authorize.py` | downstream `decide` and `explain` |
| `actrail/audit.py` | audit records and stores |
| `actrail/clock.py` | `Clock` protocol, `SystemClock`, `FixedClock` |
| `actrail/errors.py` | exception hierarchy |
| `policy/bindings.json` | impersonation bindings |
| `policy/principals.json` | principals, scopes, session limits, permissions |
| `docs/impersonation-policy.md` | normative behaviour |
| `docs/token-format.md` | token envelope and claims |
| `docs/threat-model.md` | considered escalation paths |
| `docs/design.md` | module responsibilities and invariants |

## Policy documents

A binding is `{members, role, target, conditions}`. `members` lists the
principals allowed to act as `target`. `conditions` restricts when the binding
is usable; see `docs/impersonation-policy.md` §2.

A principal entry records `kind` (`user` or `service_account`), `scopes`,
`max_session_seconds`, and the `permissions` used by downstream decisions.

## Running the tests

```
python -m pytest -q -p no:cacheprovider
```

## Signing key

Set `ACTRAIL_HMAC_KEY` in any shared environment. Without it a development
fallback key is used, which is suitable only for local runs.
