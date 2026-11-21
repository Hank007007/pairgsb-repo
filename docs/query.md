# Query string construction

This document is normative. It defines how the query component sent to a
tenant endpoint is built from integration parameters and the literal query
declared by the path template.

## 1. Sources

The query component has two sources:

* **Integration parameters** (`params`) — an ordered mapping supplied with the
  delivery call. Values may be strings, numbers, booleans, or sequences of
  such values.
* **Template query** — the literal `key=value` items written after `?` inside
  the path template. In the template these items are already percent-encoded;
  they are decoded once before use.

Both sources are untrusted. Integration parameter names and values can contain
`&`, `=`, `#`, `+`, `%`, spaces, non-ASCII text, and control characters.

## 2. Encoding

Each name and each value is encoded independently using the
`application/x-www-form-urlencoded` rules of `docs/signature.md` §4:

* Encode the text as UTF-8, then percent-encode every byte except `A-Z`,
  `a-z`, `0-9`, `-`, `_`, `.`, and `~`.
* A space is encoded as `%20`, not as `+`.
* Uppercase hexadecimal digits.
* `&`, `=`, and `#` are encoded as `%26`, `%3D`, and `%23` and therefore can
  never split or terminate a pair.
* A `+` in a name or value is encoded as `%2B`; it is not a space.

A pair with an empty name is invalid. A pair with an empty value is valid and
is serialized as `name=`.

Boolean values serialize as `true` and `false`. Null values are omitted
entirely. `None` and an empty sequence both produce no pair.

## 3. Merging and ordering

The template query and `params` are merged as follows:

1. Decode the template query once into `(name, value)` pairs, preserving
   order and duplicates.
2. Expand every sequence-valued entry of `params` into one pair per element,
   in element order; a scalar produces one pair.
3. Group all pairs by their decoded name. Names are compared byte-for-byte
   after decoding; a name from the template and the same name from `params`
   are the same key and are merged into one group. A template item is never
   silently dropped because `params` also contains that name.
4. Within a group, template pairs come first, then `params` pairs, each in
   their original relative order. Duplicate `(name, value)` pairs are kept.
5. Sort the groups by encoded name using case-sensitive bytewise ordering of
   the UTF-8 bytes. Two distinct decoded names never compare equal. If a name
   appears once, its position is determined by that ordering alone.
6. Serialize the result as `encoded_name=encoded_value` joined by `&`, with no
   leading or trailing `&`.

The template query itself must obey §2: a literal `&` separates items, the
first `=` separates name from value, and an item without `=` is a name with an
empty value. An item with an empty name is invalid.

## 4. Empty result

If the merged result has no pairs, the request target has no `?`. An empty
query (`?`) or a dangling `&` is never emitted.

## 5. Stability

The query component is computed once, before signing. The exact string that is
signed (see `docs/signature.md` §2) is the exact string placed on the wire.
No component may append to, reorder, or re-encode the query after signing.