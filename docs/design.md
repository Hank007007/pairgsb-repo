# exchangefence — design notes

These notes describe the intended behaviour of the service and how the pieces
fit together. They describe contracts and responsibilities; they do not
prescribe implementations.

## 1. Request flow

A token-exchange request moves through five stages:

1. **HTTP handling** (`exchangefence/http/handler.py`) decides which grant is
   being served, turns the raised error into an OAuth error response, and
   returns `(status, body)`.
2. **Parameter interpretation and validation** (`exchangefence/exchange/service.py`)
   checks the client, the token types, the mode, and the subject/actor tokens.
3. **Scope resolution** (`exchangefence/scopes/canon.py`,
   `exchangefence/scopes/registry.py`) normalises, resolves aliases, expands
   wildcards, and intersects the requested / subject / registered sets.
4. **Claim construction** (`exchangefence/tokens/claims.py`) computes the
   lifetime and the `act` chain and produces the claim set.
5. **Response and grant recording** (`exchangefence/exchange/service.py`,
   `exchangefence/grants/store.py`) serialises the token, records a refresh
   grant, and builds the response body.

Each stage has a single job. In particular, scope comparison belongs to the
scope layer, not to the service; the service decides *which* sets exist, the
scope layer decides *what counts as a member*.

## 2. Layering contract

### 2.1 `scopes/canon.py`

`normalize(scope)` accepts a string, a list of strings, or `None` and returns a
canonical, sorted, duplicate-free list. Normalisation resolves aliases, so the
returned entries are canonical names.

`covers(granted, requested)` answers whether every requested scope is available
from *granted*. It applies alias resolution and wildcard rules. `docs:*` covers
`docs:read`; it does not cover `docs`. Coverage is directional.

Neither function should depend on the HTTP layer or on policy.

### 2.2 `scopes/registry.py`

`resolve_alias(scope)` maps an alias to its canonical name and leaves other
names alone.

`registered_for_client(client, scopes)` is the registration check. It must
compare canonical forms, so a registration written `read` matches a request for
`docs:read`. Its result is the set of scopes the client is allowed to obtain,
expressed canonically.

### 2.3 `exchange/service.py`

`exchange(params, store=None, now=None)` orchestrates the request. It
validates the subject and actor tokens, chooses the requested scope set, asks
the scope layer for the result, and delegates claim construction and grant
recording. On error it raises the specific error type from `errors.py`; it does
not decide HTTP status codes.

`_result_scope(params, subject_claims, client)` is the one place that computes
the issued scope set. It intersects the requested set, the subject token's
granted set, and the client registration. It must reject a request whose
scope cannot be satisfied rather than quietly returning a subset.

### 2.4 `tokens/claims.py`

`build(...)` returns the claim set for the access token. Lifetime is the minimum
of subject remaining lifetime, requested `ttl` (or the default), and the
deployment maximum. The `act` chain is carried over from the subject token,
with the new actor prepended, and its depth is preserved. `may_act` is part of
the delegation decision, not of the token's own claim set.

### 2.5 `grants/store.py`

`create(...)` records a grant. `lookup(handle)` returns the current record or
raises `invalid_grant`. `rotate(handle, scopes=...)` retires the presented
handle and returns a new one whose scope is the scope recorded by the
redemption. A grant's recorded scope is what a later redemption may draw on.

### 2.6 `http/handler.py`

`handle_exchange(params, store=None)` maps grant types to service calls and maps
error types to error codes:

| Error type | Code |
|---|---|
| `InvalidRequest`, `InvalidToken` | `invalid_request` |
| `InvalidScope` | `invalid_scope` |
| `AccessDenied` | `access_denied` |
| `InvalidGrant` | `invalid_grant` |

Responses contain only the public fields of a successful result, and error
bodies contain no traceback or internal exception text.

## 3. Scope semantics

- A scope collection is unordered; normalisation is the only way to compare two
  collections.
- Aliases are resolved before any comparison. `read` and `docs:read` are the
  same scope.
- `docs:*` covers every scope that starts with `docs:`. It does not cover
  `docs`.
- A request is satisfied only if every requested scope is available from both
  the subject grant and the client registration.
- The issued token's `scope` uses canonical names.

## 4. Delegation semantics

- Mode is inferred from `actor_token` unless `mode` is given explicitly.
- Delegation requires `actor_token`; impersonation forbids it.
- `may_act` on the subject token gates the actor. Absent or mismatched
  `may_act` is `access_denied`.
- `act` is a linked chain of at most three levels. Each level preserves the
  level below it. A fourth level is `access_denied`.

## 5. Refresh semantics

- Every successful redemption rotates the handle.
- A redeemed handle is dead immediately.
- A grant's scope is the ceiling for redemption. A downscoped grant never
  restores the broader scope it was derived from.
- Grant expiry is not extended by rotation.

## 6. Error mapping

The service layer raises typed errors; the HTTP layer maps them to codes and
statuses. The mapping is stable, so a given failure produces the same code
regardless of which stage detected it.
