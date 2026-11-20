# rulemill rule language

This document is the contract for the expression language tenants use to
write alert rules. It states what a conforming evaluator must accept, reject
and return. It deliberately describes behaviour, not internals.

A rule is a single expression. When an event arrives, the platform evaluates
the rule against that event; a truthy verdict marks the event as a hit. Rules
are untrusted input. The evaluator runs inside a process that can also see
other tenants' events and the process environment, so the language is a
sandbox first and a convenience language second.

---

## 1. Lexical structure

### 1.1 Whitespace

Spaces, tabs, carriage returns and newlines separate tokens and are otherwise
insignificant.

### 1.2 Numbers

A number is a sequence of ASCII digits, optionally followed by `.` and at
least one more digit. Leading `-` is not part of a numeric literal; negation
is not in the language. `12`, `0`, `3.5` are numbers. `12.` and `.5` are not.

### 1.3 Strings

A string literal may be delimited by either `"` or `'`. Both delimiters have
identical meaning.

Inside a string, a backslash introduces an escape. The following escapes are
defined:

| Escape | Meaning |
|---|---|
| `\"` | `"` |
| `\'` | `'` |
| `\\` | `\` |
| `\n` | line feed |
| `\t` | tab |
| `\r` | carriage return |
| `\/` | `/` |

An unrecognised escape is a lexical error. A string whose closing delimiter is
missing before the end of input is a lexical error; it must never be silently
truncated, and a partial token must never reach the parser.

A string may not contain an unescaped newline.

### 1.4 Identifiers and keywords

An identifier starts with an ASCII letter or `_`, and continues with ASCII
letters, digits or `_`. No other characters — including `$`, `#`, `-`, and
non-ASCII letters — may appear in an identifier.

The keywords are `and`, `or`, `not`, `in`, `matches`, `true`, `false`, `null`.
Keywords are case sensitive and may not be used as field names or function
names.

### 1.5 Punctuation and operators

`(`, `)`, `[`, `]`, `,`, `.`, `==`, `!=`, `<`, `<=`, `>`, `>=`.

Any other character outside a string literal is a lexical error.

---

## 2. Grammar

```
expression  := or
or          := and ( "or" and )*
and         := not ( "and" not )*
not         := "not" not | comparison
comparison  := postfix ( ("==" | "!=" | "<" | "<=" | ">" | ">=") postfix
                       | "in" postfix
                       | "matches" postfix )?
postfix     := primary ( "." identifier | "(" arguments ")" )*
primary     := number | string | "true" | "false" | "null"
             | identifier
             | "(" expression ")"
             | "[" arguments "]"
arguments   := ( expression ( "," expression )* )?
```

Notes:

- Comparison is non-associative. `a == b == c` is a parse error.
- `and` binds tighter than `or`; `not` binds tighter than both.
- `in` and `matches` sit at comparison precedence.
- An identifier in primary position, when not followed by `(` or `.`, is a
  field reference.

### 2.1 Structural limits

Parsing is bounded. A rule that exceeds either limit is rejected with a
compile-time error; a stack overflow must not escape the compiler.

- Nesting depth (parentheses, array literals, function calls, `not`) is
  limited to **64**.
- The AST may contain at most **2048** nodes.

---

## 3. Values

The value domain is: `null`, boolean, number, string, array, and the opaque
CIDR block produced by `cidr()`. There are no objects, no user-defined
structures and no functions as values.

An array literal `[a, b, c]` produces an array of its evaluated elements and
is only meaningful as the right-hand side of `in`.

A field that does not exist in the event — or that is not declared for the
tenant — evaluates to `null`.

---

## 4. Field access

Field paths are dot-separated: `source.ip`, `actor.user`, `event.action`.

- Every path segment must be a valid identifier.
- Access is restricted to the **field schema** bound at compile time. The
  schema is a per-tenant, flat allow-list of full paths. A path that is not in
  the allow-list for the compiling tenant is a **compile-time** error. This
  applies to every segment, not only the first.
- The names `constructor`, `__proto__` and `prototype` may never appear in a
  field path, in any position.
- A path may only traverse the event document. It may not reach functions,
  prototypes or anything that is not plain event data. Reading a property of
  an array or of a non-event object is not supported.

Because the schema is bound at compile time, a compiled rule belongs to one
tenant. Two tenants compiling identical text must receive independent
compiled rules; neither may see the other's allow-list.

---

## 5. Functions

Only the following functions exist. Any other name in call position is a
compile-time error, as is calling a field or a property:

| Function | Arity | Result |
|---|---|---|
| `lower(s)` | 1 | lower-cased string |
| `upper(s)` | 1 | upper-cased string |
| `trim(s)` | 1 | string with surrounding whitespace removed |
| `len(v)` | 1 | length of a string or array |
| `contains(s, t)` | 2 | whether `s` contains `t` |
| `startsWith(s, p)` | 2 | prefix test |
| `endsWith(s, p)` | 2 | suffix test |
| `coalesce(a, b, ...)` | 1+ | first argument that is neither `null` nor missing |
| `int(v)` | 1 | integer value, or `null` if not numeric |
| `cidr(s)` | 1 | CIDR block, or `null` if `s` is not a valid IPv4 CIDR |
| `isCidr(v)` | 1 | whether `v` is a CIDR block |

Functions are pure. They receive no event document, no environment and no
`this`. Argument count must match; wrong arity is a compile-time error.

---

## 6. Operators

### 6.1 Equality

`==` and `!=` are **type-strict**. Values of different types are never equal.
No coercion of any kind is performed: `"1" == 1` is `false`, `null == false`
is `false`, `0 == false` is `false`.

- `null == null` is `true`.
- Numbers compare numerically; strings compare by code unit.
- Booleans compare by value.
- Arrays and CIDR blocks compare by identity only; since rules cannot
  construct the same array twice, `[1] == [1]` is `false`.

### 6.2 Ordering

`<`, `<=`, `>`, `>=` are defined for number/number and string/string pairs.
Any other combination yields `false`.

### 6.3 Logical operators

`and`, `or` and `not` are **boolean** operators. Their result is always `true`
or `false`.

- Operands are converted to booleans: `null`, `false`, `0`, `NaN` and the empty
  string are falsy; arrays — including empty arrays — and CIDR blocks are
  truthy.
- `and` evaluates its left operand first and returns `false` without
  evaluating the right operand when the left is falsy.
- `or` evaluates its left operand first and returns `true` without evaluating
  the right operand when the left is truthy.
- `not` negates the boolean of its operand.

The final verdict of a rule is the boolean of the whole expression.

### 6.4 `in`

```
value in array
value in cidrBlock
```

The right-hand side must evaluate to an array or to a CIDR block. Membership
in an array uses the same type-strict equality as `==`. Membership in a CIDR
block tests whether `value` is a dotted-quad IPv4 address inside the block.

Any other right-hand side — including a string — makes the operator `false`.
A string right-hand side is **not** a substring test; use `contains()` for
that.

### 6.5 `matches`

```
string matches pattern
```

Both sides must evaluate to strings, otherwise the result is `false`.

A pattern literal must satisfy all of the following. A pattern that does not
is a **compile-time** error. When the pattern is computed at run time, an
invalid pattern makes the operator `false`.

- Length at most **256** characters.
- It may begin with inline flags `(?i)`, `(?s)`, `(?m)` or a combination, each
  at most once, in that order, and nothing else. Flags in any other position
  are not allowed.
- It uses only the subset: literal characters, `.`, character classes,
  anchors `^ $`, groups `( )` and `(?: )`, alternation `|`, the quantifiers
  `? * + {n} {n,} {n,m}`, and escapes of a single non-alphanumeric character.
- Backreferences, lookaround, named groups, and escapes introducing character
  classes other than `\.` style punctuation escapes are not part of the
  subset.

Matching is stateless: evaluating the same rule against the same event any
number of times yields the same result, and evaluating one event cannot
change the result for the next.

---

## 7. Evaluation budget

Every evaluation of one rule against one event has a step budget of **10000**
steps. A step is charged for each AST node evaluated, each array element or
function argument evaluated, and each membership comparison.

If the budget is exhausted the evaluation stops and the rule is treated as
**not matching**. The budget is per event; it must not leak into later
evaluations.

---

## 8. Errors

Compilation failures are reported as errors with a stable code and a source
position. Categories:

| Code | Meaning |
|---|---|
| `E_LEX_UNEXPECTED_CHAR` | character not part of the language |
| `E_LEX_UNTERMINATED_STRING` | string not closed before end of input |
| `E_LEX_BAD_ESCAPE` | unrecognised escape sequence |
| `E_PARSE_UNEXPECTED_TOKEN` | token not valid here |
| `E_PARSE_UNEXPECTED_EOF` | input ended mid-expression |
| `E_PARSE_TRAILING_INPUT` | tokens after a complete expression |
| `E_PARSE_TOO_DEEP` | nesting limit exceeded |
| `E_PARSE_TOO_LARGE` | node limit exceeded |
| `E_COMPILE_UNKNOWN_FIELD` | field path not in the tenant schema |
| `E_COMPILE_UNKNOWN_FUNCTION` | name is not a whitelisted function |
| `E_COMPILE_BAD_ARITY` | wrong number of arguments |
| `E_COMPILE_BAD_PATTERN` | pattern outside the allowed subset |
| `E_UNKNOWN_TENANT` | no schema for the tenant |

A run-time failure while evaluating an event — including budget exhaustion —
never propagates to the caller: the event is simply not marked.

---

## 9. Example

Given an `acme` event

```json
{
  "severity": "high",
  "message": "login failed for alice",
  "source": { "ip": "10.1.2.3", "port": 443, "host": "edge-1" },
  "actor": { "user": "alice" }
}
```

and the rule

```
severity == "high" and source.ip in cidr("10.0.0.0/8")
```

the verdict is `true`.

With `severity == 1` the verdict is `false` (`"high"` is not the number `1`).
With `message matches "(?i)LOGIN FAILED"` it is `true`. A rule using
`source.constructor` is rejected at compile time.
