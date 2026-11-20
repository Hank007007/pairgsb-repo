# Design notes

## Position in the pipeline

```
broker ──bytes──▶ decode(payload, schema) ──dict──▶ handler
```

`decode` is the only place that reads wire bytes.  Handlers never parse
msgpack, never coerce types, and never see values outside their schema.

## Layers

| module | responsibility |
|---|---|
| `msgpack.Reader` | turn bytes into Python scalars/containers; map frames become dicts |
| `canon.normalize_key` | the comparison form for map keys |
| `schema.validate` | check a decoded value against a schema literal and build the result |
| `limits.Budget` | account for resource consumption of one message |
| `consumer.decode` | compose the above for a single message |
| `consumer.Consumer` | hold a schema (and a budget) across a stream of messages |

The reader is intentionally small and dependency-free.  All data-driven
failures are raised from `payloadcomb.errors`, so a handler wrapper can catch
`PayloadError` once and treat the message as poison.

## Data model

* Wire values become `None`, `bool`, `int`, `float`, `str`, `bytes`,
  `list`, `dict`, or `ExtValue`.  A decoded map is a plain `dict`.
* A schema is plain data — no classes, no code generation.  Declared fields
  are looked up by their normalised name; the result carries the spelling the
  schema declares.
* Validation is structural: it walks the value and the schema together and
  raises on the first violation, reporting a JSON-path-like location.

## Trust boundaries

Three properties matter for correctness and are spelled out in
`docs/payload-schema.md`:

1. **One interpretation per message.**  The bytes must mean exactly one thing
   on every side of the boundary: duplicate keys, ambiguous key spellings,
   and non-string keys have no defined "later wins" semantics.
2. **Everything the handler sees is in the schema.**  Values are either
   validated or rejected; nothing rides along unexamined.
3. **Bounded work.**  Nested input can be arbitrarily deep and wide, so the
   reader's own limits — not the interpreter's — define what is acceptable,
   and accounting is per message.

## Error taxonomy

* `PayloadError` — the inbound bytes are unacceptable (wire, schema, limits).
* `SchemaError` — the schema literal itself is wrong; this is a programming
  error and is not caught with payload failures.
* `DecodeError` subclasses — malformed or unrepresentable wire content.
* `ValidationError` subclasses — structurally valid wire content that does
  not satisfy the schema.
* `PayloadTooDeep`, `PayloadTooLarge` — resource limits.

## Observability

`examples/replay.py` is the manual check: it feeds each captured payload in
`fixtures/payloads/` through the same entry point a consumer uses and prints
the accepted value or the rejection.  Its output is meant to be compared
line-by-line with the contract.
