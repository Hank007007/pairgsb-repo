# leasefold-credlease

A small, dependency-free Python implementation of a **lease-based dynamic
credential broker**. Services ask the broker for database credentials by
*role*; the broker obtains a short-lived credential from a pluggable backend,
wraps it in a lease, and supports renewal, revocation and a per-role local
cache.

The behavioural contract lives in [`docs/lease-model.md`](docs/lease-model.md).

## Layout

```
src/credlease/
  broker.py    public API: request_lease / renew_lease / revoke
  leases.py    Lease record and the LeaseStore state machine
  policy.py    role policies, duration parsing, revisions, TTL resolution
  cache.py     local per-role lease cache
  backend.py   backend protocol + in-memory implementation
  clock.py     injectable clock (Clock / ManualClock)
tests/         happy-path tests
docs/          behavioural specification
```

## Install and run

Python 3.10+ is required. No third-party runtime dependencies are used.

```bash
python -m pip install -e .
python -m pytest -q -p no:cacheprovider
```

Or run straight from a checkout without installing — the tests add `src/` to
the path through `pyproject.toml`:

```bash
python -m pytest -q -p no:cacheprovider
```

## Quick start

```python
from credlease import Broker, InMemoryBackend, ManualClock, Policy, PolicyStore

policies = PolicyStore({
    "analyst": Policy(
        role="analyst",
        backend="db-analyst",
        max_ttl=15 * 60,      # absolute ceiling for any lease
        soft_ttl=5 * 60,      # renewal cadence
        reuse=True,
    ),
})

clock = ManualClock()
broker = Broker(policies, InMemoryBackend(), clock)
lease = broker.request_lease("alice", "analyst", ttl=2)   # 2 minutes

broker.renew_lease(lease.lease_id)
broker.revoke("alice", "analyst")
```

## Concepts

- **Lease** — a credential plus lifecycle metadata: `issued`, `active`,
  `renewed`, `revoked`, `expired`.
- **Soft TTL vs absolute expiry** — `expires_at` slides on renewal while
  `absolute_expires_at` never moves.
- **Policy revision** — every reload of policies bumps `Policy.version`; the
  cache is scoped to that revision.
- **Revocation scope** — revocation applies to every live lease a subject holds
  in a role and is propagated to the backend and to the cache.

Duration values can be integers (seconds) or strings such as `"30s"`, `"15m"`,
`"2h"`, `"1d"`. `request_lease(ttl=...)` is expressed in **minutes**, while
`request_lease(ttl_seconds=...)` is expressed in seconds.

## Backends

`backend.CredentialBackend` is a `typing.Protocol` with two methods, `issue`
and `revoke`; both raise `BackendError` on failure. `InMemoryBackend` is the
default implementation used by the tests. A real database driver can be
substituted by implementing the same two methods.
