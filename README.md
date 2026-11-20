# payloadcomb

The first hop for jobs arriving from the task queue.  Consumers pull a
msgpack-encoded job out of the broker, run it through `decode(payload, schema)`
and hand the resulting typed dict to a handler.

Everything the queue delivers is untrusted: jobs are written by other
services and by user-triggered export flows, so this module is the only
boundary between raw bytes and handler code.

* Hand-written msgpack reader, standard library only.
* Schemas are plain literals: `{"kind": "int", "min": 0, "max": 65535}`,
  `{"kind": "list", "of": {...}}`, `{"kind": "map", "keys": {...}}`.
* The contract is documented in [`docs/payload-schema.md`](docs/payload-schema.md);
  the module layout is described in [`docs/design.md`](docs/design.md).

## Layout

```
payloadcomb/
  msgpack.py    wire reader: read_map, read_str, read_int, read_ext
  schema.py     schema literals and validate / field_spec
  canon.py      map-key normalisation
  limits.py     per-message Budget
  consumer.py   decode() and the long-lived Consumer
  errors.py     exception hierarchy
examples/
  replay.py     replay captured payloads from fixtures/
fixtures/
  payloads/     captured msgpack bodies
tests/
```

## Running

```sh
python -m pytest -q -p no:cacheprovider
python examples/replay.py
```

`replay.py` prints, for each captured payload, either the mapping the handler
would receive or the reason it was rejected.  Compare that output with the
contract in `docs/payload-schema.md`.
