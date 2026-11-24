# Evaluation — check, expand, recursion, context

This document defines how a request is evaluated. The rewrite semantics
themselves live in `model.md` §2; this document covers the evaluation
machinery around them.

## 1. `check(object, relation, subject, options)`

`check` answers: is `subject` a member of `relation` on `object`, in the
caller's tenant, at or after a given point in history?

Options:

| option | meaning |
| --- | --- |
| `at_least` | a zookie; the read must observe a snapshot no older than it (`consistency.md` §2) |
| `contextualTuples` | tuples that exist only for the duration of this request (`§3`) |

The return value is a boolean, or an error. Errors are never encoded as
`false`; `false` means "evaluated successfully and the subject is not a
member".

### 1.1 Request validation

Before any tuple is read:

* `object` must be `type:id` with a syntactically valid type and id;
* `relation` must be declared on the object's namespace in the caller's
  tenant;
* `subject` must be a plain subject (`type:id` or `type:*`) — userset subjects
  are not accepted as *queries*, only inside tuples.

A violation is an `invalid_request` error (or `namespace_not_found` when the
namespace is absent, or `tenant_mismatch` when the object belongs to another
tenant's namespace — see tenancy).

### 1.2 Traversal

Evaluation walks the compiled rewrite for `(object, relation)`. Every rewrite
node either reads tuples (`this`), rewrites within the object
(`computedUserset`), rewrites across objects (`tupleToUserset`), or combines
child results (`union`, `intersection`, `exclusion`). The three combining
operators behave as described in `model.md` §2; in particular an error from a
non-short-circuited child is propagated.

Nested userset subjects (a tuple whose subject is `group:eng#member`) are
resolved by recursively evaluating `check` on the userset's object and
relation. That recursive evaluation carries the same options — notably the
same contextual tuples and the same `at_least`.

### 1.3 Recursion depth

The evaluator tracks call depth. A "call" is one evaluation of an
`(object, relation, subject)` triple, including every nested userset
resolution and every rewrite hop. The limit is **25**. Reaching depth 26 — a
chain of 26 nested evaluations — must fail with `depth_exceeded`. It must not
return `false`, and it must not be silently truncated.

Depth is per request: a sibling branch that starts after another branch has
returned starts again from the request's base depth, not from wherever the
previous branch ended.

### 1.4 Errors inside combinators

`union` may skip evaluating children once one is true. `intersection` may skip
once one is false. Those short-circuits are the *only* case in which an
erroring branch may be ignored — because it is not evaluated at all.
`exclusion`'s `subtract` branch is always evaluated when `base` is true, and
its errors are never swallowed.

A branch that is evaluated and errors contributes an error, not `false`.
Consequently:

* `union(false, error)` is an error;
* `intersection(true, error)` is an error;
* `exclusion(base=true, subtract=error)` is an error.

## 2. Cycle detection

### 2.1 Identity of a stack frame

A frame on the evaluation stack is identified by **`object#relation`** — the
object *and* the relation. `subject` is constant for a request and does not
distinguish frames. Two different relations on the same object are different
frames: evaluating `group:1#member` which computes `group:1#admin` which
computes `group:1#owner` is a chain, not a cycle.

A cycle exists when the frame `object#relation` currently on the stack is
entered again. The result is `cycle_detected`.

### 2.2 Scope

Cycle detection is per branch: the stack is pushed before a frame is evaluated
and popped after it returns. Two siblings may both evaluate the same
`object#relation` — that is not a cycle, because they are not on the stack at
the same time.

A cycle is an error, not `false`, and it propagates through combinators per
§1.4.

## 3. Contextual tuples

`contextualTuples` is an array of tuple strings (or tuple objects) supplied
with a single request. They behave exactly like stored tuples for the
duration of that request:

* they are visible to `this` wherever it is evaluated during the request —
  including inside nested userset resolution and inside `tupleToUserset`
  traversal — and are **not** visible to other requests;
* they are never persisted, never advance a zookie, and never appear in
  another tenant;
* they are not validated against the namespace registry the way stored writes
  are, but malformed entries are an `invalid_tuple` error;
* if the same tuple exists both contextually and in the store it is simply
  present once.

Contextual tuples are merged at the point of use (the tuple read), not once at
the entry point, so that every nested frame sees them exactly as it sees
stored tuples.

## 4. `expand(object, relation, options)`

`expand` returns the set of subjects that are members of `object#relation`, as
far as they can be enumerated from tuples:

* `this` contributes the plain subjects of its tuples, and, for each userset
  subject, the members of that userset, recursively (the userset itself is not
  returned as an opaque leaf when it can be expanded);
* `computedUserset` contributes the expansion of the target relation on the
  same object;
* `tupleToUserset` contributes, for each object reachable through the tupleset
  relation, the expansion of the computed relation on that object; the tupleset
  relation must be a pure `this` relation (`model.md` §2);
* `union` contributes the union of its children's expansions;
* `intersection` and `exclusion` **cannot** be enumerated from tuples alone.
  In real Zanzibar expand is undefined for them; here `expand` must report an
  error (`invalid_request` with a message naming the relation) rather than
  return a wrong or partial set.

The result is:

```json
{ "object": "doc:1", "relation": "reader", "subjects": ["user:alice", "group:eng#member"] }
```

with duplicate subjects removed and order unspecified. Userset subjects that
were fully expanded appear as their *leaf* plain subjects; a userset that
cannot be expanded further (no tuple data, i.e. empty membership) contributes
nothing.

`expand` honours `at_least` and `contextualTuples` with the same meaning as
`check`. It is not a decision and must not be cached as one.

### 4.1 Cycle and depth in expand

`expand` uses the same stack identity (`object#relation`) and the same depth
limit as `check`. `cycle_detected` and `depth_exceeded` are errors.
