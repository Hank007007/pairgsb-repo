# Manifest format — anchorveil

This document is the normative reference for the manifest syntax accepted by
`anchorveil`.  It describes the supported subset of YAML, the scalar typing
rules, the handling of duplicate keys, the anchor/alias expansion limits and
the semantics of the tag registry.  Anything not described here is not part of
the supported subset and must not be relied upon.

The loader is used for internal configuration manifests: deployment-unit alert
rules and rate-limiting parameters are rendered by the platform group, and
manifests are also received from business repositories and from external
partners.

## 1. Encoding and structure

* A manifest is UTF-8 text with a single document.  Multi-document streams
  (`---` separators) are not supported.
* Indentation uses spaces only; tab characters are a parse error.
* Blank lines and lines whose first non-space character is `#` are ignored.
  A `#` starts a comment only when preceded by whitespace or at the start of a
  line; `#` inside a quoted scalar is literal text.
* The root value may be a mapping, a sequence or a scalar.

## 2. Nodes

The loader understands four kinds of nodes:

| Node | Syntax | Example |
|---|---|---|
| scalar | a single line of text, optionally quoted | `port: 8080` |
| sequence | `- ` entries at a common indentation | `- a` |
| mapping | `key: value` pairs at a common indentation | `host: localhost` |
| tagged node | a `!`-prefixed tag applied to a node | `port: !env PORT` |

Mappings and sequences nest by indentation.  A sequence entry may itself open
a mapping (`- name: x`) whose remaining keys are indented under the dash.

A key is the text before the first `:` that is followed by end of line or a
space.  A key may be quoted with single or double quotes; quoting only affects
which characters belong to the key and does not change its value.

## 3. Scalars

### 3.1 Quoting

A scalar that begins with `'` or `"` and ends with the same quote is a quoted
scalar.  **A quoted scalar always denotes a string.**  Its value is the text
between the quotes; `''` inside single quotes denotes one apostrophe, and the
double-quoted form supports the escapes `\n`, `\t`, `\r`, `\0`, `\\` and `\"`.
No type inference is applied to quoted scalars, so:

* `"0755"` is the string `0755`, not the integer 755;
* `"no"` is the string `no`, not the boolean false;
* `"1e999"` is the string `1e999`, not infinity;
* `""` is the empty string.

### 3.2 Unquoted scalars

An unquoted scalar is typed as follows, in this order:

1. `null`, `Null`, `NULL` and `~` denote null.  An empty value is **not**
   null: it denotes the empty string.
2. `true`/`True`/`TRUE`, `yes`/`Yes`/`YES`, `on`/`On`/`ON` denote the boolean
   true; `false`/`False`/`FALSE`, `no`/`No`/`NO`, `off`/`Off`/`OFF` denote the
   boolean false.
3. A decimal integer (`[-+]?[0-9]+`) denotes an `int`, in base 10.  Leading
   zeros are preserved as a decimal integer: `0755` is 755.
4. `0x`-prefixed hexadecimal and `0o`-prefixed octal integers denote an `int`
   in the corresponding base.
5. A number containing `.` or an exponent (`e`/`E`) denotes a `float`, using
   Python `float` semantics.  Values outside the range of a double become
   `inf` or `-inf`; they are not an error.
6. Everything else is a string.

**An empty value is the empty string, whether it is quoted or not.**  Only
the null words listed above denote null; an empty value must never be turned
into `None`.

## 4. Anchors and aliases

* `&name` before a node records that node under `name`.  `*name` inserts a
  copy of the recorded node.
* An alias that names no anchor is an error.
* Anchors are scoped to the document.
* Expansion is **eager and materialised**: the value returned by the loader
  never shares structure with the anchor definition, so mutating one expansion
  must not affect another.

### 4.1 Expansion limits

Aliases can be nested, so a small input can expand to a very large tree.  A
single `load_text`/`load_file` call must therefore enforce two limits on the
**expanded** tree:

* **Absolute limit:** the expanded tree contains at most **10 000 nodes**.
* **Amplification limit:** the expanded tree contains at most **20 ×** as many
  nodes as the original input has characters.  (Input size is the character
  count of the manifest text.)

A tree is counted as: every scalar, sequence and mapping counts as one node,
and a sequence or mapping counts its children in addition to itself.  A
tagged node counts as one node plus its content.

If either limit is exceeded the load fails with an `AliasLimitError`.  The
limits are a safety property, not a best-effort hint: the loader must stop
expanding rather than allocate an unbounded tree, and must not exhibit
super-linear time or memory growth on adversarial input.

## 5. Merge key `<<`

* `<<` inside a mapping is the merge key.  Its operand is either a mapping or
  a sequence of mappings.
* Keys from the merged mapping(s) are added to the containing mapping only if
  the containing mapping does not already define them.  **An explicitly
  written key in the containing mapping always wins over merged keys.**
* When the operand is a sequence, entries are applied in order and **the
  first mapping in the sequence has the highest priority**; later entries only
  fill keys not provided by earlier entries (or by the containing mapping).
* `<<` itself is not a key of the resulting mapping.  After loading, the key
  `<<` must never appear in the returned data.
* A merge operand that is not a mapping or a sequence of mappings is an error.

## 6. Tags

A tag is written `!name` immediately before a node's value.  Tags are
resolved by the **tag registry** supplied to the loader (or the default
registry built by `default_registry`).

* Only tags present in the registry are accepted.  **A tag that is not in the
  registry is an error (`UnknownTagError`).**  The loader never interprets tag
  text as a module path, class name, callable or argument list, and loading a
  manifest must never import a module, execute code from the tag name, or
  spawn a subprocess.
* A registered tag's handler receives the node's resolved content and returns
  the replacement value.  The registry is the only extension point: how a tag
  expands is entirely up to the registry.
* The default registry defines:
  * `!env NAME` — replaced by the value of environment variable `NAME`; an
    unset variable is an error.
  * `!include PATH` — replaced by the contents of `PATH`; available only when
    loading through `load_file`, and paths are resolved relative to the
    including file.
  * `!secret NAME` — replaced by a secret looked up through the configured
    secret provider; without a provider it is an error.
* Tag handlers must not be able to cause the loader to import modules named by
  manifest content.

## 7. Duplicate keys

**A mapping may not declare the same key twice.**  If a key repeats within the
same mapping, loading fails with a `DuplicateKeyError`, regardless of the
values involved.  The loader must not silently keep the first or the last
occurrence: manifests are signed and audited as text, and a key that a
reviewer reads must be the key the runtime sees.

This applies to the merge key as well: more than one `<<` in the same mapping
is a duplicate-key error.

## 8. Errors

All errors derive from `ManifestError`:

| Error | Raised when |
|---|---|
| `ParseError` | the text does not conform to this document |
| `DuplicateKeyError` | a mapping repeats a key |
| `AliasError` | an alias has no anchor, or an alias cycle is detected |
| `AliasLimitError` | alias expansion exceeds §4.1 |
| `MergeError` | a merge operand is not a mapping or sequence of mappings |
| `UnknownTagError` | a tag is not in the registry |
| `TagError` | a registered tag handler fails or is used incorrectly |

## 9. Return value

The loader returns plain Python data: `dict[str, object]`, `list[object]`,
`str`, `int`, `float`, `bool` or `None`.  No node objects or proxy wrappers
are ever returned.
