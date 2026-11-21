# Redaction policy

The redaction policy decides, for each leaf of a record's `fields` tree,
whether the value is written verbatim or replaced by a commitment.

## 1. Policy file

A policy is a JSON document:

```json
{
  "rules": [
    {"path": "user.password", "action": "redact"},
    {"path": "items[].ssn", "action": "redact"},
    {"path": "user.id", "action": "keep"}
  ]
}
```

Each rule has:

| Member | Required | Meaning |
| --- | --- | --- |
| `path` | yes | a path pattern (§2) |
| `action` | no | `redact` (default) or `keep` |

Rules are evaluated in document order. The **first** rule that matches a field
decides that field's action; later rules are not consulted.

## 2. Path grammar

A field path is built from object keys and array indexes.

- Object keys are joined with `.`: `user.password`, `a.b.c`.
- An array position is written with an empty bracket, `[]`, in a **rule
  pattern**: `items[].ssn` means "the `ssn` key of every element of the array
  `items`".
- A concrete field path — the one the matcher sees while walking a record —
  carries the actual index, for example `items[0].ssn`, `items[1].ssn`.

Rule patterns match against concrete paths. A `[]` segment in a pattern
matches exactly one array index segment in the concrete path, whatever the
index. Nesting is allowed: `accounts[].owners[].email`.

A pattern without `[]` matches only scalar leaves whose path is identical to
the pattern; it does not match a sibling whose path merely begins with the
pattern text.

## 3. Matching rules

### 3.1 Exact path segments

Pattern segments are compared segment by segment. A pattern matches only when
every one of its segments matches the corresponding segment of the concrete
path, and the two paths have the same number of segments.

In particular, a rule for `user.password` matches `user.password` and nothing
else. It does not match `user.password_hint`, `user.password2`, or
`userXpassword`. A rule for `user` matches nothing unless `user` is itself a
scalar leaf.

### 3.2 Arrays

A `[]` pattern segment matches any single array index at that position. The
rule `items[].ssn` matches all of `items[0].ssn`, `items[1].ssn`,
`items[2].ssn`, and so on for every element present in the record. It does
not match `items[0].nested.ssn` and does not skip levels.

### 3.3 Case

Path comparison is **case-insensitive** with respect to ASCII letters. The
pattern `user.password`, `User.Password` and `USER.PASSWORD` all match the
concrete path `User.Password`. Case folding applies only to ASCII letters;
non-ASCII characters are compared exactly. The canonical path recorded in the
journal keeps the key's original spelling.

### 3.4 Default action

When no rule matches a leaf, the action is **`redact`**. A policy whose rule
list is empty therefore redacts every leaf. Choosing to keep a field on the
record must always be an explicit `keep` rule; silence never means "write the
plaintext".

This default is a safety property: forgetting to add a rule may hide a field
from readers, but it must never publish one.

## 4. What counts as a leaf

- An object member whose value is an object or array is not itself a leaf;
  matching applies to the leaves below it.
- Every scalar — string, number, boolean, `null` — is a leaf and is matched
  independently.
- A value already holding a commitment is not redacted again.

## 5. Interaction with the journal

A leaf whose action is `redact` is replaced by a commitment object (§3 of
`journal-format.md`) before the record is hashed and written. A leaf whose
action is `keep` is encoded verbatim, and its canonical value participates in
the hash input as plaintext.

## 6. Policy errors

A policy document that cannot be decoded, contains a rule with an empty
`path`, or names an unknown `action` is rejected at load time. A rejected
policy is an error, never a silent fallback to another policy.
