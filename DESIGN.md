# anchorveil design notes

This note records the behaviour the loader is expected to have and the shape
of the implementation.  The normative syntax and typing rules live in
[`docs/manifest-format.md`](docs/manifest-format.md); this document does not
repeat them, it explains how the pieces fit together.

## Scope

`anchorveil` reads a deliberately small subset of YAML for internal
configuration manifests.  It is not a general YAML implementation: no flow
collections, no multi-document streams, no complex keys, no tag syntax beyond
`!name`.  Keeping the subset small is what makes the security-relevant rules
(expansion limits, tag registry, duplicate keys) tractable.

## Pipeline

```
text
  │
  ├─ parser.py      → node tree + anchor table
  ├─ alias.py       → node tree with aliases materialised
  ├─ tags.py        → node tree with registered tags expanded
  ├─ merge.py       → node tree with `<<` folded into mappings
  └─ loader.py      → plain dict / list / scalars
```

The parser never produces Python data directly.  A scalar keeps its source
text plus a `quoted` flag, an alias stays symbolic, and `<<` stays in the
mapping as an ordinary key until the merge stage.  Type resolution happens
once, at the very end.

The order is deliberate:

1. Aliases are materialised before tags and merges, so a tag or merge
   operand sees ordinary nodes.
2. Tags are expanded before merges, so a tag may return a mapping that is
   then used as a merge operand.
3. Materialisation is last, so every rule above sees the same intermediate
   form.

## Behaviour contracts

### Syntax

Indentation is spaces only.  Mappings are `key: value`; sequences are `- `
entries.  Quoted keys are allowed and quoting only delimits the key text.  A
`#` starts a comment only when preceded by whitespace or at the start of a
line; quoted scalars are exempt.

### Scalars

Quoting is semantic: a quoted scalar is always a string, with no type
inference.  An unquoted scalar is null, boolean, integer, float or string
according to the ordered rules in the format document.  An empty value is an
empty string, not null; only the explicit null words denote null.

### Aliases

An alias inserts an independent copy of the anchored node.  Expansion is
materialised: the result never shares mutable structure between two
expansions or with the anchor definition.  An alias naming no anchor, and an
alias cycle, are errors.

Expansion is bounded.  A load whose expanded tree would exceed either the
absolute ceiling or the multiple-of-input ceiling must fail with
`AliasLimitError`, and must do so promptly — a small adversarial input must
not be able to force allocation or work proportional to the full expansion.
The bounds themselves are stated in §4.1 of the format document.

### Tags

Tags are named `!name`.  The registry is the entire extension mechanism and
the only source of tag semantics.  A tag absent from the registry is an
error.  Tag text is never interpreted as a module path, attribute, callable
or argument list; loading must not import modules or execute code chosen by
the manifest.

The default registry provides `!env`, `!include` and `!secret`.  `!include`
is resolved while loading a file, before alias expansion, and contributes
its text to the input-size accounting; nested includes are bounded.

Handlers receive resolved content and return a plain value.  Handlers that
need the environment, files or a secret provider get them from the closure
the registry was built with, not from the manifest.

### Merge keys

`<<` takes a mapping or a sequence of mappings.  Explicitly written keys in
the containing mapping take priority over every merged key.  In a sequence,
earlier entries take priority over later ones.  `<<` is consumed and never
appears in the result.

### Duplicate keys

A key may appear at most once in a mapping.  A repeat is a load error; the
loader must not pick a winner silently.  This includes a repeated `<<`.

### Errors

Every failure is a `ManifestError` subclass, as listed in §8 of the format
document.  Stages raise their own subclasses and the loader does not
translate them, so a caller can distinguish a parse problem from an alias
limit, an unknown tag or a bad merge operand.

## Constraints

* Standard library only.  No third-party YAML parser.
* Floats are Python `float`; no custom precision handling.
* The public entry points are `load_text` and `load_file`; their signatures
  and return shapes are stable.
* The tag registry's registration API is stable.

## Testing

`tests/` covers the normal path: single documents, mappings and sequences,
quoted text, unquoted scalars, registered tags and file loading with
`!include`.  The suite is a smoke test for the pipeline; it is not an
exhaustive conformance suite for the format document.
