# Policy compilation

This document describes how a document accepted by `confseal.intake` becomes
the policy object handed to the filter engine. It is the contract for
`confseal.policy`.

## 1. Shape

The document is a mapping. It has an optional `version` and a required
`rules`.

```yaml
version: 1
rules:
  - id: allow-partner-a
    subject: team-a
    action: allow
    destinations:
      - partner-a.example
    schedule:
      from: "09:00"
      until: "17:00"
```

`version` defaults to `1` when absent. A version that is not an integer is an
error.

## 2. Rules

`rules` is a sequence of mappings. Each mapping is one rule. It must have:

| field | type | meaning |
|---|---|---|
| `id` | non-empty string | identifies the rule |
| `subject` | non-empty string | who the rule applies to |
| `action` | `allow` or `deny` | what happens to matching traffic |
| `destinations` | non-empty sequence of non-empty strings | where traffic may go |

It may have:

| field | type | meaning |
|---|---|---|
| `schedule` | mapping | when the rule is in force |

A `schedule` mapping may have `from` and `until`, both non-empty strings
written as `HH:MM`. Any other field in a schedule is an error, as is a
schedule that is a sequence or a scalar.

`<<` inside a rule mapping or inside a schedule mapping means what
`intake.md` §2.5 says it means everywhere: the merged entries are added, and
an entry written explicitly beside the merge key wins over the merged one.

Fields that are not listed above are carried through to the compiled rule
unchanged, under `extra`, rather than being rejected: policy files are
written by hand and older fields are kept for the filter engine to ignore.

## 3. Compiling

### 3.1 Input

Compilation takes the tree returned by intake. It may also be handed the text
directly, in which case intake runs first.

### 3.2 Rule identity

Rule ids are compared without regard to case. `Deny-All`, `deny-all` and
`DENY-ALL` are the same id. Two rules in one document whose ids match that
way are a duplicate-id error, and there is no way to opt out of the
comparison.

The id is stored as written; it is the comparison that ignores case.

Every other field is taken as it was written. A key written as `on` is the key
`"on"`, and a value written as a plain, quoted or tagged scalar is the value
that scalar denotes.

## 4. The compiled object

### 4.1 Rule order

The compiled rules appear in the same order as the rules in the document.
Compilation never sorts, never groups, and never reorders by id or by any
other field. The filter engine evaluates rules in the order it receives them,
so the order in the file is meaningful and is preserved exactly.

### 4.2 Shape

`compile_policy` returns a `Policy`:

| member | type | content |
|---|---|---|
| `version` | int | the document version |
| `rules` | sequence of `Rule` | in document order |

Each `Rule` has:

| member | type | content |
|---|---|---|
| `id` | str | as written |
| `subject` | str | as written |
| `action` | str | `allow` or `deny` |
| `destinations` | sequence of str | as written, in order |
| `schedule` | mapping | `from` / `until`, or empty |
| `extra` | mapping | every other field, as written |

`Policy.to_dict` and `Rule.to_dict` produce the JSON-ready form of the same
data.

## 5. Failures

### 5.1 Error type

Compilation raises `PolicyError`, carrying a stable `code` and, where one
applies, a line. Codes used by compilation include:

| code | cause |
|---|---|
| `invalid_document` | the document is not a mapping |
| `invalid_version` | the version is not an integer |
| `invalid_rules` | `rules` is missing or is not a sequence |
| `invalid_rule` | an entry of `rules` is not a mapping |
| `invalid_merge` | a merge value is not a mapping or sequence of mappings |
| `missing_field` | a required field is absent or empty |
| `invalid_action` | the action is not `allow` or `deny` |
| `invalid_destinations` | `destinations` is missing, not a sequence, or holds a non-string |
| `invalid_schedule` | the schedule is malformed |
| `duplicate_rule_id` | two rules share an id |

### 5.2 Positions

The line reported for a problem inside a rule is the line on which that rule
mapping begins. The line reported for a problem with the document as a whole
is the line on which the document mapping begins.

### 5.3 Determinism

Compiling the same tree twice yields equal policies. Compilation reads no
clock, no environment and no file.

### 5.4 Response

A document that compiles is returned in full. A document that does not
compile is reported with:

* the error's `code`,
* the error's `line`, when the error has one, and
* a short description of the problem that names no part of the submitted
  text.

The submitted text, any line of it, and any fragment of it must not appear in
the response. The response never contains a traceback, a reader's own
multi-line rendering of the failure point, or a copy of the failing value.
