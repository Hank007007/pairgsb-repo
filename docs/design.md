# Design notes

`exprseal` evaluates tenant-authored boolean rules against the current request.
This document records the intended behaviour of the engine and the shape of
its internals. `docs/expression-spec.md` is the normative description of the
language; `docs/field-map.md` is the normative list of readable paths. Both
take precedence over the code.

## Pipeline

```
text ──► lexer ──► tokens ──► parser ──► AST ──► evaluator ──► value
```

Each stage has a single job and returns plain data:

| Stage | Module | Produces |
|---|---|---|
| lexing | `exprseal.lexer` | `Token(kind, value, pos)` |
| parsing | `exprseal.parser` | `nodes.*` dataclasses |
| evaluation | `exprseal.eval` | Python values (`bool`, `str`, `int`, `list`, ...) |

The public surface is `parse`, `evaluate`, and `Context`. `parse` returns a
root node; `evaluate` takes that node and a `Context`.

## Isolation

Expressions are untrusted input. The engine therefore:

1. never calls `eval`, `exec`, `compile`, or `__import__`;
2. exposes only two root names, `user` and `request`;
3. reads only the paths in the field map;
4. invokes only the functions in the whitelist, in both call forms.

Anything outside that surface is a *denial* (`AccessDenied`), not a host
exception. An `AttributeError` or `TypeError` from the host runtime must not
reach the caller of `evaluate`.

## Correct behaviour at a glance

* **Precedence.** `not` applies to the whole comparison expression on its
  right, so `not a in [x]` negates the membership test. `in` binds looser than
  `==`/`!=` and tighter than `and`. The full table is in the specification.
* **Membership.** `x in y` is defined only when `y` is a list; it tests
  equality against each element. A string on the right is an error. There is
  no substring special case.
* **Field access.** A path is allowed if and only if it appears in the field
  map. Underscore-prefixed segments are never in the map, so they are always
  denied.
* **Calls.** `foo.bar(...)` is allowed only when `bar` is a whitelisted
  function name, and it means `bar(foo, ...)`. Merely having a method of that
  name on the object is not sufficient, and having a non-whitelisted method is
  not permission.
* **Limits.** Source size, token count, AST depth, and evaluation steps are
  hard ceilings. Size, token, and depth checks happen before evaluation
  starts; the step budget is decremented as the evaluator walks the tree.
  Exceeding any of them raises `LimitExceeded`.
* **Short-circuiting.** `and` and `or` evaluate the right-hand side only when
  the left-hand side does not already decide the result.

## Error model

`ExpressionError` is the base class. `ParseError`, `LimitExceeded`, and
`EvaluationError` derive from it, and `AccessDenied` derives from
`EvaluationError`. A caller can catch `ExpressionError` and treat a rule as
unusable without inspecting host exceptions.

## Context

`Context(user, request)` holds the request-scoped values. `lookup(name)`
resolves root names; `resolve(obj, name)` reads a field. Both are the only
places where host objects are touched, which keeps the policy for names and
fields in one module.
