# Policy intake

This document describes what confseal accepts when an operator submits a
policy file, and what it must reject. It is the contract for
`confseal.intake`.

## 1. Scope

Intake covers one text: the body of an upload request. Its job is to turn that
text into a plain tree of mappings, sequences and scalars that the policy
layer can consume. Intake says nothing about what the fields mean; that is
`policy.md`.

Two properties matter more than any single feature:

* the tree intake returns must contain only values the operator actually
  wrote, in the shape they wrote them; and
* the work intake performs must stay proportional to the size of the text and
  the size of the tree it produces.

## 2. Reading the text

### 2.1 One upload, one text

The upload body is a single UTF-8 string. Intake does not fetch anything,
does not merge in defaults, and does not consult any other file.

### 2.2 Permitted node kinds

A policy document may contain only:

* mappings,
* sequences, and
* scalars.

A scalar may be written plain, single-quoted, double-quoted, or block-style. A
tag that the reader does not recognise is an error. This includes application
tags such as `!sealed`, and the language-specific tags such as
`!!python/object`, `!!python/name` and `!!python/object/apply`. Intake must
never drop an unknown tag and carry on with the underlying value: an operator
who pasted a fragment carrying a tag must be told that the fragment cannot be
accepted.

A tag is unknown unless it is one of the standard tags for the kinds above
(string, integer, float, boolean, null, mapping, sequence, and the merge key
described in §2.5). This holds wherever the tagged value is written,
including inside a value that a reference reaches.

### 2.3 Scalars

A scalar keeps the value its tag denotes. Quoted scalars are always strings,
whatever they look like. A plain scalar that reads as a number, a boolean or
a null is converted to that value when it appears as a *value*.

### 2.4 Duplicate keys

Within one mapping, the same key may not be written twice. This applies at
every depth: a mapping nested inside a sequence nested inside another mapping
is held to the same rule as the top-level mapping.

A repeated key is an error, and the position reported is the line of the
later occurrence. Intake does not silently keep the first value, does not
silently keep the last value, and does not warn.

The one exception is described in §2.5.

### 2.5 Merge keys

`<<` merges the entries of one mapping, or of each mapping in a sequence of
mappings, into the mapping that carries the merge key.

* A key that arrives only through a merge is not a duplicate of anything.
* If a key arrives through a merge and is also written explicitly in the
  same mapping, that is not a duplicate either, and the explicitly written
  entry wins.
* If two different merge sources provide the same key, the source that
  appears earlier in the merge value wins.
* The merge key's value must be a mapping or a sequence of mappings; anything
  else is an error at the point the merge is resolved.

Intake keeps the merge key in the tree and does not expand it: deciding what
the merged entries are, and which of two contenders wins, belongs to the
policy layer (`policy.md` §2). Duplicate detection at intake therefore only
ever compares keys written literally in the same mapping.

The rules above must hold identically whether the merged mapping is written
inline or reached through an alias.

### 2.6 Keys are strings

Every mapping key in the tree is a string, regardless of how it was written.
A key written as `on`, `off`, `yes`, `no`, `true`, `false`, `1`, `1.0` or `~`
is the exact text the operator typed: `"on"`, `"off"`, `"yes"`, `"no"`,
`"true"`, `"false"`, `"1"`, `"1.0"`, `"~"`. It is not a boolean, a number or
a null.

The same text is the same key: a mapping containing both `on:` and `"on":`
has a duplicate key and is an error under §2.4.

Keys that are themselves mappings or sequences are not permitted.

## 3. Aliases

Anchors and aliases are part of the input language and are accepted. When
intake is done, no alias remains: every reference has been replaced by a
value of its own. Writing a value once and referring to it many times is
therefore not a way to keep the tree small — the tree that reaches the policy
layer is the fully written-out one.

Intake reads the text without interpreting aliases first, so a text whose
fully expanded form would be enormous is still read quickly; whether it is
accepted is decided in §3.1.

### 3.1 Expansion limits

After expansion the tree must satisfy both limits:

* the total number of nodes (mappings, sequences, and scalars, counting every
  copy produced by a reference) is at most **10 000**; and
* a reference may not be nested inside more than **4** levels of references —
  that is, following the chain of aliases from an outer reference to the
  innermost one may not take more than four steps.

Both limits are global to the document. They are not per top-level entry, not
per branch, and not per mapping.

A document over either limit is rejected. It must be rejected before the tree
has been built at a size that is expensive to build: a short text that
expands to hundreds of thousands of nodes must not first allocate hundreds of
thousands of nodes and then be discarded.

The limits are on the expanded tree, so an anchor that is never referenced
contributes its nodes once, while an anchor referenced five times contributes
its nodes five times. A document with many anchors that are each referenced
once is therefore held to the same 10 000-node ceiling as any other: the
ceiling is on the tree, not on the text.

### 3.2 Anchors are local to the document

An anchor may only be referenced within the document that defines it. A
reference to an anchor that was never defined is an error.

## 4. The result

`load_document` returns the expanded document.

The result is a tree of `dict`- and `list`-like containers holding scalar
values. No value is shared between two places in the tree: a container that
appeared once in the text appears once in the result. Containers record the
line on which they started in the submitted text so that later layers can
report positions.

## 5. Failures

### 5.1 Codes and positions

Every rejection carries:

* a stable code identifying the kind of problem, and
* the line in the submitted text where the problem was found, or nothing if
  the problem is not tied to one line.

Codes used by intake include:

| code | cause |
|---|---|
| `invalid_yaml` | the text is not well-formed |
| `unsupported_tag` | the text carries a tag intake does not accept |
| `duplicate_key` | one mapping writes the same key twice |
| `node_limit` | the expanded tree exceeds 10 000 nodes |
| `alias_depth` | a reference nests more than four levels deep |
| `multiple_documents` | the text holds more than one document |
| `empty_document` | the text holds no document, or an empty one |

Lines are counted from 1 and refer to the submitted text as sent, including
any blank lines and comments.

### 5.2 What a message may say

A failure message may name the problem, the field and the position. It must
not reproduce the submitted text. Intake errors reach the caller through the
same error type the policy layer uses, so the API layer can report them
without inspecting the exception's class.

### 5.3 One document

The submission is exactly one document. A second document introduced by
`---` is an error. Intake never ignores trailing content, never returns the
first document and discards the rest, and never merges documents together.

An empty submission is an error.

### 5.4 Reading errors

Text that is not well-formed yields an error positioned at the point where
reading failed. The position is the one the reader reports, adjusted to be
counted from 1.
