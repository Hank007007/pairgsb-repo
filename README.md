# blobrestore

`blobrestore` is a sidecar service for a shared object store. Producer
processes serialize their results into a *chunked blob* and write it to the
store; a consumer later asks `blobrestore` to turn the blob back into ordinary
Python objects.

The blob format exists so that blobs written by other services, other
versions, and other machines can be read back without handing the decoder the
ability to construct arbitrary objects. It supports a type table, shared
references (one copy of a sub-structure, one object on the way out), and an
`ext` section for third-party payloads.

## Layout

```
blobrestore/
  wire/
    format.py     header parsing and chunk framing
    restore.py    decoding a chunk stream into an object graph
  model/
    typemap.py    the type table and type-id resolution
    reencode.py   canonical re-encoding for cache keys
  service.py      revive(), logging, HTTP shell
  ext.py          the __bytes__ ext wrapper
docs/
  blob-format.md  normative byte layout
  service.md      normative service behaviour
tests/            pytest suite
testdata/         sample blobs
tools/            testdata generator
```

## Quick start

```
python -m pytest -q -p no:cacheprovider
```

```python
from blobrestore.service import revive

result = revive(blob_bytes)
if result["ok"]:
    value = result["value"]
else:
    print(result["error"])
```

## Reading order

Start with `docs/blob-format.md` for the byte layout and `docs/service.md` for
budgets, isolation, logging, and the restore pipeline. The modules follow the
same order: header, type table, chunk stream, refs, re-encode.

## Design notes

* `read_header` is the first thing that runs; everything downstream assumes
  the offsets it returns are usable.
* `load_type_table` runs before the chunk stream is decoded, so a table that
  names a disallowed type is rejected before any object exists.
* `restore` decodes records in stream order and resolves refs against the
  records it has already produced.
* `reencode` produces the bytes the cache key is derived from; the key must be
  a function of the graph, not of the process that built it.

## Scope

Standard library only. The HTTP shell in `service.py` is a thin wrapper over
`revive`; deployments put their own front end in front of it.
