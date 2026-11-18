# Expression specification

This document is normative. Wherever the implementation and this document
disagree, this document wins. The expression language is intentionally small:
it is written by tenants in a back-office console and evaluated on every
request, so every construct below is part of the public contract.

The engine never uses `eval`, `exec`, `compile`, or `__import__`.

## 1. Source text

An expression is a single UTF-8 string.

| Limit | Value | Enforced |
|---|---|---|
| Maximum source size | 512 bytes | before tokenizing |
| Maximum number of tokens | 2048 | after tokenizing, before parsing |
| Maximum AST depth | 32 | after parsing, before evaluation |
| Maximum evaluation steps | 10 000 | during evaluation |

Exceeding any limit raises `LimitExceeded`. Length, token-count, and depth
violations are detected before any part of the expression is evaluated.
Evaluation-step exhaustion stops the evaluation and raises `LimitExceeded`.

## 2. Lexical elements

* Identifiers: `[A-Za-z_][A-Za-z0-9_]*`.
* Keywords: `and`, `or`, `not`, `in`, `true`, `false`, `null`.
* Integers: decimal digits, e.g. `0`, `42`.
* Floats: decimal digits with a fractional part, e.g. `1.5`.
* Strings: single- or double-quoted. Escapes are `\n`, `\r`, `\t`, `\\`,
  `\'`, `\"`. A newline inside a string is an error.
* Operators and punctuation: `==`, `!=`, `<`, `<=`, `>`, `>=`, `+`, `-`,
  `*`, `/`, `%`, `.`, `(`, `)`, `[`, `]`, `,`.

Any other character, a bare `=` or `!`, an unterminated string, or an
unsupported escape is a `ParseError`.

## 3. Grammar and precedence

Precedence, from lowest to highest:

| Level | Operators | Associativity |
|---|---|---|
| 1 | `or` | left |
| 2 | `and` | left |
| 3 | `not` (prefix) | right |
| 4 | `in` | left |
| 5 | `==`, `!=` | left |
| 6 | `<`, `<=`, `>`, `>=` | left |
| 7 | `+`, `-` | left |
| 8 | `*`, `/`, `%` | left |
| 9 | unary `+`, unary `-` | right |
| 10 | `.name`, `(args)`, `[items]` | postfix |

Consequences that follow directly from the table:

* `not` is a prefix operator whose operand is the **entire comparison
  expression to its right**. Therefore `not a in [x]` means `not (a in [x])`,
  and `not a == b` means `not (a == b)`. Use parentheses when a narrower
  operand is intended: `(not a) in [x]`.
* Membership has lower precedence than equality, so `a == b in c` means
  `(a == b) in c`.
* `and` binds tighter than `or`; both short-circuit.

Parenthesized expressions are allowed at any position. A list literal is
written `[expr, expr, ...]`; trailing commas are not allowed. An empty list
`[]` is valid.

## 4. Evaluation semantics

* `and` evaluates the left operand first. If it is falsy, it is returned
  unchanged; otherwise the right operand is evaluated and returned.
* `or` evaluates the left operand first. If it is truthy, it is returned
  unchanged; otherwise the right operand is evaluated and returned.
* `not` converts its operand to a boolean and negates it; the result is a
  boolean.
* `==` and `!=` compare values with value equality. `!=` is the negation of
  `==`.
* `<`, `<=`, `>`, `>=` require operands that support ordering; incompatible
  operand types raise `EvaluationError`.
* Arithmetic uses the usual numeric meaning. Division or modulo by zero, and
  arithmetic on incompatible types, raise `EvaluationError`.
* Unary `-` and `+` require a number; otherwise `EvaluationError`.
* The value of a list literal is a new list containing the evaluated items.

### 4.1 Membership

`left in right` is true when `right` is a list and at least one element of
`right` equals `left` under `==`. The left operand may have any type.

The right operand **must** be a list. Any other right operand, in particular
a string, raises `EvaluationError`. In no case does `in` perform substring,
prefix, or otherwise string-specific matching. `"min" in "admin"` is an
error, not a substring test.

## 5. Names, fields, and calls

### 5.1 Root names

The only root names are `user` and `request`. Any other root name raises
`AccessDenied`.

### 5.2 Field access

A property path is a root name followed by zero or more `.name` segments.
Only the paths listed in `docs/field-map.md` may be read. Reading any other
attribute — including any attribute whose name begins with an underscore —
raises `AccessDenied`.

Denials are reported as `AccessDenied`. A Python `AttributeError`, `TypeError`,
or other host-language exception must never escape the engine.

### 5.3 Function calls

Two call forms are accepted:

* `name(arg, ...)` where `name` is one of the whitelisted functions below.
* `value.name(arg, ...)` where `name` is one of the whitelisted functions.
  This form is exactly equivalent to `name(value, arg, ...)`.

| Function | Meaning |
|---|---|
| `startswith(value, prefix)` | `str(value)` starts with `str(prefix)` |
| `endswith(value, suffix)` | `str(value)` ends with `str(suffix)` |
| `contains(value, needle)` | `needle` is contained in `value` |
| `lower(value)` | lower-cased `str(value)` |
| `upper(value)` | upper-cased `str(value)` |
| `len(value)` | number of items in `value` |

The whitelist applies to both forms. `foo.bar(...)` is rejected unless `bar`
is one of the names above; it is **not** sufficient for `bar` to exist on the
object. Calling a non-whitelisted method, calling a whitelisted name with the
wrong arity, or calling anything that is not a whitelisted function raises
`AccessDenied` or `EvaluationError` — never a host-language exception.

## 6. Errors

All engine errors derive from `ExpressionError`:

| Exception | Raised when |
|---|---|
| `ParseError` | the source is not a well-formed expression |
| `LimitExceeded` | a size, token, depth, or step limit is exceeded |
| `EvaluationError` | a well-formed expression cannot produce a value |
| `AccessDenied` | a name, field, or call is not permitted |

`AccessDenied` is a subclass of `EvaluationError`. In particular, an
expression that tries to read a hidden attribute or invoke a non-whitelisted
method is denied, not crashed: the caller only ever sees errors from the table
above.

## 7. Examples

```
user.plan == "pro" and request.path.startswith("/api")
user.role in ["admin", "owner"]
not user.verified
(len(user.groups) > 2) or user.plan == "enterprise"
lower(user.email) == "ops@example.com"
```

The following are invalid and must be rejected:

```
user.role in "admin"          # right operand of `in` must be a list
user.__dict__                 # attribute not in the field map
request.path.split("/")       # call to a non-whitelisted method
user.plan ==                  # incomplete expression
```
