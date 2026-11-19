# mergeknot-config — merge rules

This document is normative. It describes what a conforming implementation must
do. It deliberately does not describe how any of it is implemented.

Throughout, "the document" means the merged configuration object that `load()`
returns, and "a patch document" means the array passed to `applyPatch()`.

---

## §1 Layer order

Layers are applied from lowest precedence to highest precedence:

1. `defaults` — the caller-supplied base object.
2. `base.json` — the first entry of `layers`.
3. Environment overlays — every remaining entry of `layers`, in array order.
4. Environment variables carrying the configured prefix.
5. `--set a.b=c` entries from the command line, in the order given.

A later layer overrides an earlier one. Overriding is recursive: when both
sides hold a plain object at the same path, the objects are merged key by key
rather than replaced. Any other combination (scalar over object, object over
scalar, array involved) replaces the earlier value outright.

Layers are applied in the order above regardless of the order in which the
caller lists `base.json` relative to the overlays; the first `layers` entry is
always the base.

### §1.1 Arrays

Arrays do not merge element-wise. When the incoming layer supplies an array at
a path where the current document holds an array, the incoming array replaces
the existing one. Appending is expressed through the JSON Patch interface
(§5.4), not through layers.

### §1.2 Purity

`load()` must not mutate any of its inputs. The objects reachable from
`defaults`, from the parsed layer files, and from any caller-supplied argument
must be unchanged after the call, and must not share mutable substructure with
the returned document. Mutating the returned document must not change the
caller's defaults.

### §1.3 Determinism

Merging the same inputs twice, in either order of unrelated keys, must produce
deeply equal documents. Key iteration order of an input object must not change
the outcome.

---

## §2 Key whitelist

Only keys that are safe to use as ordinary object property names may appear in
the document. A key is admissible when all of the following hold:

- It is a non-empty string.
- It is not a reserved key (§3).
- It contains no control characters (U+0000–U+001F, U+007F).
- It does not contain the path separator used by the surface that produced it
  (`.` for dotted paths and environment variables, `/` for JSON Patch
  pointers).

The whitelist applies uniformly to every way a key can enter the document:
a parsed JSON layer, an environment variable name, a `--set` path, a JSON
Patch operation, and the target of a `copy`/`move`.

A key that fails the whitelist is a rejected key. A rejected key must never be
written into the document, must never be created as an intermediate container,
and must not cause the rest of its layer to be discarded silently — it is
reported through the error channel (§7).

Names are not normalised, with the single exception of environment-variable
segments (§4.1). No trimming is performed, and `A` and `a` are distinct keys
everywhere else.

---

## §3 Reserved keys

The following keys are reserved and may never appear at any depth of the
document, in any container, and regardless of the layer that supplies them:

| Key | Reason |
| --- | --- |
| `__proto__` | rewires the prototype chain when assigned |
| `constructor` | reaches `Object` through the prototype chain |
| `prototype` | reaches `Function.prototype` through a constructor |
| `length` | aliases array length and breaks array semantics |

Reserved keys are a subset of rejected keys (§2) but are reported with their
own error code because the distinction matters to callers: a reserved key is an
attempt to reach outside the document, while any other rejected key is a
malformed input.

A reserved key supplied by *any* input source — including but not limited to
`base.json`, an overlay, an environment variable, a `--set` path, or a patch
operation, and including a reserved key nested inside an otherwise valid
object — is rejected. Rejecting a nested key drops only that entry; the other
keys of the same object are still applied.

---

## §4 Environment variables

Environment variables are selected by prefix (§1, step 4). The prefix itself is
removed from the name; a name that does not start with the prefix is ignored.

### §4.1 Nesting

A double underscore (`__`) in the remaining name denotes a path separator.
`APP_DB__POOL__SIZE` therefore addresses the document path `db.pool.size`.
A single underscore is part of the key name and is not a separator.

Each resulting segment is lower-cased before it is matched against the
document, because environment variable names are conventionally upper case
while configuration keys are not. This is the only place where case folding
happens; nothing else about the name is rewritten.

An empty segment — produced by a leading, trailing, or doubled separator
adjacent to another separator — is a rejected key (§2) and must not be
silently collapsed into a neighbouring segment. A name whose segments would
spell a reserved key (§3) is rejected with `E_RESERVED_KEY`.

### §4.2 Type coercion

The target key's existing value in the document determines the type the
environment string is coerced to. The rule is:

| Existing value at the target path | Coercion |
| --- | --- |
| boolean `true` / `false` | case-insensitive `true` / `false`; other spellings are `E_TYPE` |
| number | the numeric value of the string; non-numeric or non-finite input is `E_TYPE` |
| array | the string is appended as a single element, coerced per §4.3 |
| `null` | the string is kept as-is |
| absent | the string is kept as-is |
| anything else | the string is kept as-is |

Examples that must hold:

- With `db.pool.size` defaulting to `4`, `APP_DB__POOL__SIZE=8` yields the
  number `8`, not the string `"8"`.
- With `features` defaulting to an array, `APP_FEATURES=beta` appends `"beta"`.
- With `enabled` defaulting to `true`, `APP_ENABLED=FALSE` yields `false`,
  and `APP_ENABLED=maybe` is `E_TYPE`.

### §4.3 Array element coercion

When the target path holds an array, each appended element is coerced using
the element type of the array's first element, following the same table as
§4.2. An empty array has no element type, so the string is appended unchanged.
Elements are appended, never merged into an existing element.

### §4.4 Missing containers

If the document has no value at the target path, the environment variable
creates the intermediate containers needed to reach it. Those containers are
plain objects and are subject to §2 and §3 like any other key.

---

## §5 JSON Patch

`applyPatch(document, patch)` accepts an RFC 6902 style patch array and returns
the resulting document together with an error list (§7). Operations are
applied in array order. The input document is not modified; the returned
document is a fresh structure.

### §5.1 Supported operations

`add`, `remove`, `replace`, `move`, `copy`, and `test` are supported. Any other
`op` value is `E_PATCH_OP`. A missing or non-string `path` is `E_PATCH_PATH`.

### §5.2 Pointer syntax

A `path` (and the `from` of §5.5) is a JSON Pointer:

- It must be a string starting with `/`.
- It is split on `/` into segments *after* removing the leading separator. A
  `/` inside a key is escaped as `~1`, and a `~` as `~0`; those escapes are
  decoded before the key is used.
- The `-` segment is meaningful only as the final segment of an `add` target
  and only when the container at that point is an array (§5.4). Elsewhere it is
  a rejected key.
- The empty string is the whole-document pointer, and is legal only where §5.3
  says so.

### §5.3 Containment

A path must stay inside the document root. The following are all `E_PATCH_PATH`:

- A relative path that does not start with `/`, including the empty string
  except for the whole-document operation of §5.6.
- Parent traversal: any segment that decodes to `.` or `..`.
- A path that resolves above the root, e.g. `/../x`, `/a/../../x`.
- A path whose intermediate segment does not exist, except under `add`, which
  creates missing plain-object intermediates (§5.4).
- A path containing a reserved key (§3) at any depth, e.g. `/__proto__/y`.
  That is `E_RESERVED_KEY`, not `E_PATCH_PATH`.

The same checks apply to `from` for `copy` and `move`, and a failing `from` is
reported with the same codes.

### §5.4 Arrays

Within an array, a segment is a canonical non-negative integer, or `-` for an
appending `add`. Anything else — including `-1`, `01`, `1.0`, `length`, and
`x` — is `E_PATCH_PATH`. Indices are not sign-tolerant.

For `add` into an array, the element is inserted before the element currently
at the index, or appended when the index equals the current length or the
segment is `-`. A gap index (greater than the current length) is
`E_PATCH_PATH`.

`add` may create intermediate containers for a pointer whose parent is
missing, but only plain objects: a missing intermediate is created as `{}`,
and a non-object intermediate is `E_PATCH_PATH`. Intermediate segments are
still subject to §5.2 and §5.3.

### §5.5 `move` and `copy`

`move` removes the value at `from` and inserts it at `path`; `copy` duplicates
it. `from` must exist and must resolve inside the root, under §5.2 and §5.3.
It may not be the document root. It is `E_PATCH_PATH` for a malformed pointer
and `E_PATCH_NOT_FOUND` when the resolved value is absent. A `move` where
`path` is a descendant of `from` is `E_PATCH_PATH`.

### §5.6 Root replacement

`add` or `replace` with `path` `""` replaces the whole document. The
replacement must be a plain object; an array or scalar is `E_PATCH_TYPE`. The
resulting document is still subject to §2 and §3, and the returned document
keeps `Object.prototype` as its prototype.

### §5.7 `test`

`test` compares the value at `path` with `value` using deep structural
equality. A mismatch is `E_PATCH_TEST`. A missing path in a `test` is
`E_PATCH_NOT_FOUND`.

---

## §6 Document hygiene

The object returned by `load()` and by `applyPatch()` is read directly by
service code, so it must be an ordinary, clean object:

- Its prototype is `Object.prototype`. It is not a class instance, not a
  null-prototype object, and not an array.
- Every container reachable from it is likewise a plain object or a plain
  array. No functions, symbols, `Date`s, `Map`s, or class instances appear in
  the document.
- No own property of any container is a reserved key (§3).
- Applying any input must leave the global object untouched. After a load or a
  patch, `({}).polluted` is `undefined` for every key the input attempted to
  introduce, and `Object.prototype` has no new own properties.

JSON input cannot express a function or a symbol, but an environment variable
value or a programmatic `defaults` object can. Values that are not JSON
values (functions, symbols, `bigint`, class instances) are `E_TYPE`.

---

## §7 Errors

Every rejection is reported, never swallowed. `load()` returns
`{ config, errors }`; `applyPatch()` returns `{ document, errors }`. `errors`
is an array of `{ code, path, message }`:

| `code` | Meaning |
| --- | --- |
| `E_RESERVED_KEY` | a reserved key (§3) appeared in a key position |
| `E_INVALID_KEY` | a key failed the whitelist for a reason other than §3 |
| `E_TYPE` | a value could not be coerced to the required type |
| `E_PATCH_OP` | unsupported patch operation |
| `E_PATCH_PATH` | malformed, escaping, or non-existent-intermediate pointer |
| `E_PATCH_NOT_FOUND` | the target of `remove`, `replace`, `move`, `copy`, or `test` is absent |
| `E_PATCH_TEST` | a `test` operation compared unequal |
| `E_PATCH_TYPE` | the replacement value has the wrong structural type |

`path` is a dotted path for layer, environment, and `--set` errors, and a JSON
Pointer for patch errors. It identifies the offending entry as precisely as
the input allows. An error does not abort the remaining layers or operations
unless the operation cannot be applied at all, in which case the document
keeps the value it had before that operation.

The process must not throw for any of the above; they are data, not
exceptions. A malformed JSON layer file, or a layer file that cannot be read,
is `E_TYPE` with the file path as `path`.

---

## §8 Command line

`--set path=value` entries are parsed from `argv` in order. The path uses the
same dotted syntax as §2 and §3; the value is coerced exactly as §4.2
describes, against the document as it stands after the environment layer.

`--set` with no `=`, an empty path, or an empty key segment is `E_INVALID_KEY`.
A reserved key in the path is `E_RESERVED_KEY`. A later `--set` for the same
path wins over an earlier one.
