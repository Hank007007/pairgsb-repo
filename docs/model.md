# Model — namespaces, rewrites, tuples

tupleresolve stores authorization data in two layers:

* **Namespace configuration** — a small document per object type that says how
  each relation on that type is computed.
* **Tuples** — the facts: `object#relation@subject`.

Everything else (`check`, `expand`) is derived from those two layers.

## 1. Namespace configuration

A namespace document is JSON:

```json
{
  "name": "doc",
  "version": 3,
  "relations": {
    "viewer": { "union": [
      { "this": {} },
      { "computedUserset": { "relation": "editor" } }
    ] },
    "editor": { "this": {} },
    "parent": { "this": {} },
    "owner": { "tupleToUserset": {
      "tupleset": { "relation": "parent" },
      "computedUserset": { "relation": "owner" }
    } }
  }
}
```

* `name` — the object type this document describes. It must match the type
  part of every object the document is used to evaluate.
* `version` — a monotonically increasing integer chosen by whoever writes the
  document. It identifies *which* configuration produced a compiled result.
  A reload that produces a new version is a new configuration even if the
  relations happen to be identical.
* `relations` — a map from relation name to a rewrite expression.

Relation names match `[a-z][a-z0-9_]*`. Object types match
`[a-z][a-z0-9_]*`. Object ids match `[A-Za-z0-9_.\-]+`.

### 1.1 Rewrite expression grammar

A rewrite expression is an object with **exactly one** operator key.

| Operator | Shape | Meaning |
| --- | --- | --- |
| `this` | `{ "this": {} }` | consult the tuples stored for this object and relation |
| `computedUserset` | `{ "computedUserset": { "relation": "r" } }` | evaluate relation `r` on the same object |
| `tupleToUserset` | `{ "tupleToUserset": { "tupleset": { "relation": "r" }, "computedUserset": { "relation": "s" } } }` | follow tuples in relation `r` to other objects, then evaluate `s` on each |
| `union` | `{ "union": [ expr, ... ] }` | disjunction |
| `intersection` | `{ "intersection": [ expr, ... ] }` | conjunction |
| `exclusion` | `{ "exclusion": { "base": expr, "subtract": expr } }` | difference |

Any other operator key, a missing operator key, two operator keys in the same
expression, a malformed operand (non-object where an object is required,
missing `relation`, non-array where an array is required) is an **invalid
namespace document**. Loading such a document must fail loudly with an error
that names the namespace and the offending relation; it must never be
interpreted as some other operator. In particular there is no silent fallback
and no "unknown means union" rule.

`union` and `intersection` accept zero or more children. An empty union is
`false`; an empty intersection is `true`.

### 1.2 Compilation

Compiling turns a namespace document into an immutable plan. The plan records
the namespace name, the configuration version, and for every relation an
operator-tagged node. Compilation is pure: the same document compiled twice
yields plans that behave identically. The compiler must reject unknown
operators and unknown relation references (`computedUserset.relation` or
`tupleToUserset` operands that name no relation in the same document).
Reference cycles that pass through tuples, usersets or rewrite hops are
legitimate in the configuration and are diagnosed at evaluation time
(`evaluation.md` §2), not at compile time.

Compiled plans are cached. The cache key is the namespace name **together with
the configuration version**. A compiled plan may only be reused for the exact
configuration version it was produced from; after a reload that changes the
version, evaluation must use a plan built from the new document.

## 2. Rewrite semantics

Semantics are defined for a triple `(object, relation, subject)` and are
always interpreted within one tenant (§ tenancy).

* **union** is true iff at least one child is true. Children may be evaluated
  in any order; the result is false iff every child is false. An error raised
  by a child that is not short-circuited away must be propagated — a union
  over an erroring branch is not false.
* **intersection** is true iff every child is true. A false child short-circuits
  to false; if no child is false, an error raised by a child is propagated.
* **exclusion** is `base` and not `subtract`. It is true iff `base` is true and
  `subtract` is false. The `subtract` branch is not optional: if evaluating it
  raises an error (recursion limit, cycle, malformed data), the whole
  evaluation raises that error, even when `base` is already true. A `subtract`
  branch may never be treated as "false" because it failed.
* **computedUserset** evaluates the named relation on the same object and
  subject.
* **tupleToUserset** (TTU): read the tuples stored for the object in the
  `tupleset.relation`, take the *object* each tuple points at, and evaluate
  `computedUserset.relation` on each of those objects for the same subject.
  The result is a disjunction over all such objects. Tuples whose subject is a
  userset (`type:id#relation`) or a wildcard are not usable as TTU targets and
  are skipped. TTU is only defined over a tupleset relation whose rewrite is
  exactly `{"this": {}}`; a tupleset relation that computes anything else is
  an invalid configuration and evaluation must fail rather than guess.
* **this** is defined in §2.5.

Errors are values with a `code`:

* `namespace_not_found`, `invalid_namespace`, `invalid_tuple`, `invalid_request`
* `cycle_detected`
* `depth_exceeded`
* `tenant_mismatch` (see tenancy)
* `tuple_store_error`

The first error encountered by a non-short-circuited branch is the error
returned. Implementations may choose which of several simultaneous errors is
reported but must not replace an error with a boolean.

## 2.5. `this` — direct tuples only

`this` consults **only** the tuples stored for exactly this object and
relation. It never consults the namespace configuration of the subject, and it
never rewrites through a different relation of the same object. It is true iff
one of the following holds for the tuple set `S` of `object#relation`:

1. a tuple `object#relation@subject` exists whose subject is exactly the
   queried subject; or
2. a tuple `object#relation@type:id#relation2` exists — a *userset subject* —
   and the queried subject is a member of `type:id#relation2`.

Membership in case 2 is resolved recursively with the same evaluation rules as
`check` on `(type:id, relation2, subject)`, including depth and cycle limits.
A userset subject may itself contain userset subjects; resolution continues
until a plain subject matches or the tuple set is exhausted.

`this` is also the only place where a direct tuple can satisfy a request: a
`computedUserset` or `tupleToUserset` never reads tuples of the relation it is
declared on. A relation whose rewrite is `this` and whose tuple set is empty is
false for every subject.

Contextual tuples (§ evaluation.md §3) are part of the tuple set `S` for
`this`; they are not written to the store and they do not affect anyone else's
evaluation.

## 3. Tuples

A tuple is the string `object#relation@subject`:

* `object` = `type:id` (or just `type` for the singleton object `type:*` when a
  namespace defines it; ordinary object ids always carry `:`).
* `relation` is a relation declared on that object's namespace.
* `subject` is either a plain subject `type:id` / `type:*`, or a userset
  `type:id#relation`.

Writing a tuple requires that the object's namespace exists **in the caller's
tenant** and declares both the relation and, for a userset subject, the
subject's relation. Writes that violate this are rejected; they are never
silently stored.

Tuples are stored per tenant. The same textual tuple written by two tenants
describes two unrelated facts.

## 4. Zookies

Every successful mutation (insert **or** delete) advances the tenant's data
version and returns a zookie token that encodes a position in history. A
zookie is opaque to callers: it can be passed back as `at_least` and compared
for ordering by the store, but callers must not parse it.

The guarantee is: a read performed with `at_least = Z` observes all mutations
that completed — and therefore were assigned versions — before `Z` was
issued, and none that were assigned versions strictly later. See
`consistency.md`.

## 5. Namespace registry

A tenant has its own set of namespaces. Registering a namespace:

* validates the document (see §1.1) before storing it;
* assigns/increments its `version` if the caller does not supply one;
* makes the new version visible to subsequent evaluations in that tenant only.

A namespace name may be registered again with new content; the version must
increase. The registry keeps the current document per `(tenant, name)`; it
does not need to retain old documents for evaluation, but any cached compiled
plan must not survive a version change.
