# Tenant configuration rules

This document is the normative description of how a tenant-uploaded
configuration document is read, validated and merged into the platform
defaults. It states required behaviour only; it does not prescribe an
implementation.

Terminology: *document* is the file the tenant uploads. *Overlay* is the
mapping that the merge step applies on top of the platform defaults.
*Default configuration* is the process-wide built-in configuration in
`cfgfold/defaults.py`.

## §1 Document formats

1.1 A document is YAML by default. A document whose first non-whitespace
character is `{` is treated as JSON.

1.2 Both formats are subject to every rule in this document. A rule stated
for "a document" applies equally to the YAML and the JSON route.

1.3 The root of a document is either a mapping or a sequence. Any other root
(including a bare scalar) is rejected with `invalid-document`. An empty
document is defined to be an empty mapping.

1.4 The document root is the only place where a sequence is acceptable as
the whole document. The overlay used by the merge step (§8) must be a
mapping; a document whose root is a sequence is rejected with
`invalid-document` before merging.

1.5 Mapping keys must be scalars. A collection used as a key is rejected
with `invalid-document`.

## §2 Node model and tags

2.1 Only three node kinds are part of the accepted model: scalars,
sequences and mappings.

2.2 A tag that falls outside that model — every application-defined or
otherwise non-standard tag, such as `!!python/object/apply`,
`!!python/name:*`, `!include`, or any locally declared tag — is rejected
with `unsupported-tag`.

2.3 Rejection under §2.2 must happen while the document's structure is still
being inspected, before any value is produced from the document's tags.
Parsing a rejecting document must not execute a tag constructor, must not
instantiate a class named by the document, and must not import a module
named by the document. After the rejection, the process state must be
exactly what it was before the document was read, apart from the rejection
itself.

2.4 Values in an accepted document must be strings, integers, floats,
booleans, null, sequences or mappings.

## §3 Document size

3.1 A document's UTF-8 encoding may not exceed 256 KiB (262144 bytes).

3.2 A larger document is rejected with `too-large` before any other
processing.

## §4 Duplicate keys

4.1 Within a single mapping of a document, every key may appear at most
once. A second occurrence of the same key is rejected with
`duplicate-key`.

4.2 Key identity is defined over scalar keys: two keys are the same when
they have the same tag and the same value. The rule applies at every
mapping in the document, not only at the root, and applies to the JSON
route as well as the YAML route.

4.3 `duplicate-key` is rejected. A document with a repeated key must never
be accepted with either the first or the last occurrence silently winning.

## §5 Nesting depth

5.1 The document root counts as level 1. A collection that is contained
directly in a collection is one level deeper than its container. Scalars do
not introduce a level.

5.2 A document in which any collection sits at level 17 or deeper is
rejected with `too-deep`.

5.3 The limit counts every kind of collection, not only sequences. A chain
of nested mappings is bounded by exactly the same limit as a chain of
nested sequences.

## §6 Aliases and the node budget

6.1 A document may expand, through aliases, to at most 10,000 nodes. The
count is over the document as the merge step would observe it: the root
counts as one node, every mapping key and every value counts as one node,
and every node reached more than once through an alias counts once for
each time it is reached. A node that is not aliased counts once.

6.2 A document whose expansion exceeds 10,000 nodes is rejected with
`alias-limit`.

6.3 The limit must be enforced during the expansion itself, not after it.
Processing must be abandoned as soon as the count exceeds 10,000, so that a
document with an alias whose expansion is exponential or unbounded is
rejected promptly and cannot exhaust memory or time.

## §7 Reserved keys

7.1 The key `internal` is reserved to the platform. The reservation covers
the whole subtree below it: tenants may not add, replace or remove any
value there.

7.2 If the tenant overlay contains the key `internal` in any mapping, at
any level, the document is rejected with `reserved-key`.

7.3 The rule is about what the tenant supplies. The reserved subtree that
the platform defaults already contain is not affected by a rejection and is
always part of the effective configuration.

## §8 Merge semantics

8.1 The effective configuration is the platform defaults with the tenant
overlay applied by a deep merge.

8.2 When both the default and the overlay hold a mapping under the same
key, the two mappings are merged recursively. Keys that exist only in the
default are kept; keys that exist only in the overlay are added.

8.3 In every other combination the overlay value replaces the default
value: a scalar replaces a scalar or a mapping, a mapping replaces a
scalar, and a sequence replaces a sequence. Sequences are never
concatenated, zipped or otherwise combined; the overlay sequence is taken
whole.

8.4 A null value in the overlay is an ordinary value and replaces the
default value.

8.5 The merge is not permitted to mutate the platform defaults; see §9.

## §9 Isolation between loads

9.1 A load must leave the platform defaults exactly as it found them. After
`load_tenant_config` returns, the default configuration must still
serialize byte-for-byte to the same document it serialized to before the
call: no key added, changed, removed or reordered anywhere in the tree.

9.2 The returned effective configuration must not share mutable containers
with the platform defaults. A caller that mutates the returned mapping must
not change the defaults and must not affect any later load.

9.3 Loads are independent: a value applied by one tenant's load must never
appear in the effective configuration of a different tenant's later load
unless that value is in the platform defaults or in that tenant's own
overlay.

9.4 Rule §9.1 applies to every load, including loads that end in rejection.

## §10 Rejection codes

Every rejection is reported with one stable code. The message text is free
form; the code is part of the interface. All codes are reported through
`cfgfold.errors.ConfigError`, whose `code` attribute carries the value below
and whose string form begins with `cfgfold: <code>: `.

| Condition | Code |
|---|---|
| Document exceeds 256 KiB (§3) | `too-large` |
| YAML syntax error | `malformed-yaml` |
| JSON syntax error | `malformed-json` |
| Root is neither mapping nor sequence, overlay is not a mapping, or a mapping key is not a scalar (§1, §8) | `invalid-document` |
| A tag outside the accepted node model (§2) | `unsupported-tag` |
| A mapping repeats a key (§4) | `duplicate-key` |
| Any collection at level 17 or deeper (§5) | `too-deep` |
| Expansion exceeds 10,000 nodes (§6) | `alias-limit` |
| The overlay contains the reserved key `internal` (§7) | `reserved-key` |

A document that violates more than one rule reports one of the applicable
codes. §3.2 and §2.3 impose ordering constraints that always take priority:
an oversize document is rejected before it is inspected, and a document with
a rejected tag is rejected before any value is constructed from it.
