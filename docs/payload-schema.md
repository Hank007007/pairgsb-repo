# Payload schema

Every message in the job queue is msgpack.  Before a handler sees it the
consumer runs it through `decode(payload, schema)`.  This document defines the
contract that boundary is expected to meet.  It specifies behaviour only; the
implementation lives in `payloadcomb/`.

## 1. Schema literals

A schema is plain data.  Each node is a mapping with a `kind` key:

| kind | options | value it admits |
|---|---|---|
| `map` | `keys`: mapping of field name → schema | a msgpack map |
| `list` | `of`: schema (default `{"kind": "any"}`) | a msgpack array |
| `str` | `max_len`: maximum length in Unicode code points | a msgpack string |
| `int` | `min`, `max`: inclusive bounds | a msgpack integer |
| `float` | — | a msgpack float; an integer is accepted and converted |
| `bool` | — | a msgpack boolean |
| `ext` | `code`: the extension code expected | a msgpack extension value |
| `any` | — | any decoded value |

Field entries inside a `map`'s `keys` accept two extra options:

* `required` — defaults to `true`.  A required field must be present.
* `default` — value supplied when an optional field is absent.  Optional
  fields without a `default` are simply omitted from the result.

Example:

```python
JOB = {
    "kind": "map",
    "keys": {
        "job_id":    {"kind": "str", "max_len": 64},
        "priority":  {"kind": "int", "min": 0, "max": 9},
        "dry_run":   {"kind": "bool", "required": False, "default": False},
        "tags":      {"kind": "list", "of": {"kind": "str", "max_len": 32}},
        "deadline":  {"kind": "ext", "code": 1},
    },
}
```

## 2. Result shape

`decode` returns a plain `dict` (nested lists/maps are plain too).  It contains
**only** keys declared by the schema at that map level.  Undeclared keys are
discarded, never forwarded to the handler.

## 3. Map keys

* A map key must be a msgpack **string**.  Binary, integer, float, boolean,
  nil, array, map, and extension keys are invalid; the message is rejected
  with an error naming the offending key.
* Keys are compared after Unicode **NFC** normalisation; case is ignored.
  The pre-composed form `"café"` and the decomposed form
  `"café"` denote the same field, and so do `"Job_ID"` and `"job_id"`.
  A payload that carries both spellings carries the same key twice.
* Duplicate keys — equal after normalisation — are an error.  The error must
  identify the key.  There is no last-write-wins.
* Result keys use the spelling declared in the schema, not the spelling found
  on the wire.

## 4. Strings

* String bytes must be valid UTF-8.  Invalid sequences are rejected; they are
  never replaced with U+FFFD.
* `max_len` counts Unicode code points after decoding.
* Strings nested inside lists/maps are subject to the same rules.

## 5. Integers

* An integer decoded from the wire must fit in the range representable by a
  signed 64-bit integer (`-2**63 .. 2**63-1`).  `uint64` values above that
  boundary are rejected at the decode boundary.
* Schema `min`/`max` are inclusive and are checked for every integer field.
* Booleans are not integers.  `true`/`false` never satisfy an `int` schema,
  even though Python's `bool` subclasses `int`.

## 6. Floats

* `NaN` and both infinities are rejected.  The handler only receives finite
  values.

## 7. Extension types

Only two extension codes are accepted:

| code | name | meaning |
|---|---|---|
| 1 | `EXT_TIMESTAMP` | queue insertion timestamp |
| 2 | `EXT_TRACE` | opaque trace context |

Anything else — including any code with a `pickle` payload — is rejected with
`UnsupportedExt`.  No extension payload is ever deserialised into a live
Python object.  Accepted extensions are surfaced as `ExtValue(code, data)`.
If the schema declares a `code`, the value's code must match.

## 8. Resource limits

Measured per message, not per consumer lifetime:

* **Depth** — container nesting is at most 8.  Exceeding it raises
  `PayloadTooDeep`; the parser must never rely on Python's own recursion limit.
* **Elements** — at most 5000 elements in total, counted across every nested
  list, map, and string (strings count their length).  Exceeding it raises
  `PayloadTooLarge`.

The limits are enforced while the message is being read, so an over-limit
message is not fully validated first.  A `Budget` reused by a `Consumer`
starts each message from zero, including after a rejected message.

## 9. Errors

All data-driven failures derive from `PayloadError`; malformed schema literals
raise `SchemaError`.  Rejections happen in `decode`, before the handler runs.
