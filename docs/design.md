# tupleresolve — design notes

This document describes how the pieces of tupleresolve fit together and where
each responsibility lives. Behavioural requirements live in the four
specification documents (`model.md`, `evaluation.md`, `consistency.md`,
`tenancy.md`); this file explains the intended shape of the implementation.

## Layering

```
HTTP handlers            src/http/*.js
      │  parse, authenticate, map errors to status codes
      ▼
Service                 src/service.js
      │  binds (runtime, tenant); owns write-time namespace ownership checks
      ▼
Evaluator               src/graph/check.js, src/graph/expand.js
      │  walks compiled rewrites; recursion, cycles, contextual tuples
      ▼
Compiler                src/model/compile.js
      │  namespace document -> immutable plan, cached per version
      ▼
Stores                  src/store/tuples.js, src/store/consistency.js
      │  tuple rows, mutation clock, zookies
      ▼
Cache                   src/cache/decisions.js
```

The direction of dependency is one-way. The evaluator never parses JSON and
never writes storage; the store never walks rewrites; the HTTP layer never
decides semantics.

### Why the split

The two hard problems — "what does this rewrite mean" and "what does the data
look like right now" — change for different reasons and are tested
separately.

* **Compilation** turns a document into a plan once. Anything that can be
  checked statically (operator shape, relation references) is checked there.
  Anything that needs data (does a tuple exist? is this a cycle?) belongs to
  evaluation.
* **Tuple reading** is the only place that knows about snapshots and tenants.
  Everything above it sees "the subjects of `object#relation` at this
  snapshot", already merged with contextual tuples.
* **Evaluation** owns recursion depth, the cycle stack and operator
  composition. These are properties of the *walk*, not of the configuration
  and not of the storage, so they cannot be pushed down.
* **The decision cache** sits above evaluation and must therefore know the
  full identity of a decision: not just the triple, but the configuration
  versions and the snapshot floor that made the answer true.

## Data flow of a `check`

1. The HTTP handler validates request shape and tenant/principal agreement,
   then calls the service.
2. The service delegates to `check(runtime, tenant, object, relation,
   subject, options)`.
3. `check` parses and validates the triple, resolves `at_least` to a numeric
   floor, and asks the registry for the compiled plan for the object's type.
   A missing namespace is reported before the cache is consulted.
4. The decision cache is consulted. A hit is usable only when the tenant,
   namespace versions and snapshot floor all match the current request.
5. On a miss, evaluation starts at depth 0 with an empty cycle stack. Each
   frame pushes `object#relation`, evaluates one rewrite node, and pops.
6. `this` reads the store at the request's snapshot, merges contextual
   tuples, and resolves userset subjects by recursing.
7. The result is stored in the cache with the snapshot version observed and
   the namespace versions consulted.

## Depth and cycles

Depth counts *frames*: every `(object, relation)` evaluation, including
rewrite hops on the same object. The limit is 25. Exceeding it is a hard
failure (`depth_exceeded`) rather than a deny, because a silent `false` would
turn a configuration mistake into "deny", which is invisible in audit logs.

The cycle stack is keyed by `object#relation`, because that is the identity of
a computation. Two relations on the same object are a legitimate chain, e.g.
`group:1#member → group:1#admin → group:1#owner`. Only re-entering a frame that
is currently on the stack is a cycle. The stack is per branch and unwound
after the frame returns so siblings do not see each other.

## Errors versus booleans

The evaluator has three outcomes per node: true, false, and error. Errors are
`AuthError` values that bubble until a combinator either short-circuits them
away (by not evaluating the branch at all) or propagates them.
`docs/evaluation.md` §1.4 fixes the composition table.

## Contextual tuples

Contextual tuples are request-scoped facts. They are parsed once at the entry
point into the same shape as stored rows, then carried in the evaluation
context so that every frame reads them the same way. They are merged at the
read, never written to the store and never counted in the snapshot version.

Because they are part of the request rather than of the stored data, two
requests that differ only in contextual tuples are different requests; the
decision cache key must be able to tell them apart (see `consistency.md` §3).

## Consistency

The store keeps each row with `createdVersion` and `deletedVersion`. A read
at `asOf = v` includes rows with `createdVersion <= v` whose
`deletedVersion` is `null` or `> v`. Both inserts and deletes advance the
tenant clock and mint a token.

The decision cache records `snapshotVersion` — the store clock at the moment
the decision was computed — and treats an entry as stale when that is below
the request's floor. A request without `at_least` has a floor of "now", so any
mutation makes prior entries stale.

## Tenancy

Tenancy appears in three places and they must agree:

* storage keys include the tenant;
* the namespace registry is per tenant, and a write is checked against the
  *writing* tenant's registry;
* the decision cache key includes the tenant (or, equivalently, each tenant
  gets its own cache).

A tuple that points at an object of a type not registered in the caller's
tenant contributes nothing, which makes cross-tenant traversal
indistinguishable from "no such object".

## Non-goals

* Persistent storage. The store is in-process; a restart loses tuples and
  zookie history.
* Watch / changelog streams, Leopard indexing, and the full Zanzibar
  consistency protocols.
* Distributed caching. The decision cache is local to a runtime.
* A management UI. Namespaces and tuples are administered over the HTTP API.

## Testing strategy

The suite uses `node:test` and drives the service directly. Fixtures under
`test/fixtures/namespaces/` are small, realistic namespace documents; each
test registers the fixtures it needs into an isolated runtime so tests never
share state. A test that needs a shape the fixtures do not provide registers
an inline document — that keeps the fixture directory representative of what
production ships rather than a catalogue of edge cases.

The evaluator is the part that deserves the most direct tests: one test per
operator, one per combinator outcome (true / false / error), plus the
traversal limits and the tenancy boundaries.
