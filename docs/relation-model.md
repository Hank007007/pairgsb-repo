# Relation model

This document states the semantics `relgraph` is expected to implement. It is
normative: each section defines behaviour that callers may rely on. It says
nothing about how the evaluator is structured internally.

Throughout, an authorization question has the form
`check(subject, relation, object)`:

- **object** is a typed reference such as `doc:1` or `folder:7`.
- **relation** is a relation name declared for that object's type in
  `schema/<type>.json`.
- **subject** names the principal whose access is being tested. It is either a
  concrete principal (`user:alice`) or a userset (`group:eng#member`).

The result is a `Decision` carrying `allowed: bool` and `reasons: tuple[str, ...]`.
`reasons` is diagnostic metadata and never changes the verdict.

## §1 Relations and rewrite nodes

A schema document declares one `object_type` and a map of `relations`. Each
relation maps to a rewrite node whose `kind` is one of:

| kind | meaning |
|---|---|
| `direct` | the relation is stored explicitly by tuples |
| `union` | disjunction of the node's `children` |
| `intersection` | conjunction of the node's `children` |
| `exclusion` | `base` holds and `subtract` does not |
| `computed_userset` | the same question is asked against another relation on the same object |
| `tuple_to_userset` | the question is asked against a relation on the object reached through `tupleset` |

Schema files are data. The evaluator must not special-case any particular
relation name, object type, or combination of nodes.

## §2 Union

`union` holds if **at least one** child holds. A union denies only when every
child denies. Children are evaluated independently; the order in which they are
visited must not change the verdict.

## §3 Intersection

`intersection` holds only if **every** child holds. A single failing child is
enough to deny, regardless of how many other children hold. An intersection
with no children is not a valid schema and is rejected at load time.

## §4 Exclusion

`exclusion` holds if and only if its `base` holds **and** its `subtract` does
not. The result of evaluating `subtract` is always part of the verdict: a
subject that satisfies `subtract` is denied even when it satisfies `base`.
Evaluation must not let a successful `base` short-circuit the `subtract`
branch.

## §5 Direct tuples and userset subjects

A `direct` relation is satisfied by stored tuples of the form
`<object>#<relation>@<subject>`. The subject side may be:

- a concrete principal, e.g. `user:alice`; or
- a userset reference, e.g. `group:eng#member`.

A userset subject grants access to **every principal that belongs to that
userset**, not to a principal whose name happens to appear inside the
reference. For `group:eng#member`, the members are the subjects of the tuples
`group:eng#member@...`; members may themselves be userset references, and
membership is followed transitively. In other words, an authorization question
about `user:alice` succeeds against a stored subject `group:eng#member` exactly
when `user:alice` is a member of `group:eng`.

The subject of a `check` question names a concrete principal. Expansion applies
to the subject side of stored tuples; a stored userset reference must never be
mistaken for a concrete principal with the same identifier.

## §6 Parent traversal (`tuple_to_userset`)

A `tuple_to_userset` node with `tupleset: "parent"` and
`computed_userset: "viewer"` means: collect the objects referenced by the
object's `parent` tuples, and ask the same subject/relation question against
`viewer` on each of them. The node holds if any such parent object grants the
question. The `#relation` suffix on a parent tuple is not part of the object
reference used for the follow-up question.

## §7 Depth limit

Evaluation is bounded to **25** levels of relation expansion. A single question
counts as one level each time the evaluator follows a `tuple_to_userset` parent
or a userset membership edge. When the bound would be exceeded, evaluation
stops and returns `allowed=False` with `"depth_exceeded"` in `reasons`; it must
never raise `RecursionError` or otherwise propagate a stack failure to the
caller.

## §8 Cycles

Tuple graphs are not guaranteed to be acyclic. When relations point back at
themselves — directly or through an intermediate object — evaluation must
terminate and deny. A cycle encountered within the depth bound is reported the
same way as §7: `allowed=False`, with `"depth_exceeded"` in `reasons`. Returning
a cached intermediate result for a node that is currently being evaluated is not
permitted, because that result is not yet known.

## §9 Write visibility

The tuple store is the single source of truth. After any successful add or
remove, the **next** `check` call must reflect the new tuple set. In particular,
removing `doc:1#viewer@user:bob` and immediately checking
`check("user:bob", "viewer", "doc:1")` must deny. No decision computed against
an earlier state of the store may be returned.

## §10 Unknown input

A relation that is not declared for the object's type yields `allowed=False`.
Malformed tuple text, unknown object types, and unknown schema node kinds are
rejected when they are submitted or loaded rather than being interpreted
loosely.
