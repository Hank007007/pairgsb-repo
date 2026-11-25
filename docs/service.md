# blobrestore service

This document is normative for the service layer that wraps the blob reader
described in `blob-format.md`.

## 1. What the service does

`blobrestore` sits beside the object store. Producer processes serialize their
results into a chunked blob (`blob-format.md`), write it into the shared store,
and later a consumer asks `blobrestore` to turn the blob back into ordinary
Python objects.

The blobs are not trusted:

* they are written by several services and several versions of those services;
* they cross the network before they are stored;
* anybody with write access to the store can put arbitrary bytes there.

The service must therefore treat every blob as hostile input. It is the single
place in the fleet that decides which objects may come into existence on the
restore side.

## 2. Budgets and guarantees

### 2.1 Decode memory budget

One revive call may hold at most **64 MiB** of decode state. The budget is
charged against *declared but not yet read* data: the moment a length field
says "the next *n* bytes are mine", *n* counts against the budget, and it is
released as the bytes are actually consumed.

Consequences:

* A blob that declares a 3 GiB chunk is rejected before a single byte of that
  chunk is read, and before any buffer of the declared size is allocated.
* The limit does not apply only to `len(blob)`. A blob of a few hundred bytes
  may declare gigabytes of payload; the declared figure is what is measured.
* The budget covers the whole revive call, including refs that are decoded
  more than once because they are referenced from several places.

### 2.2 Isolation

Decoding must not import modules, evaluate names, call attributes, or construct
anything named by the blob. The only values the restore side may produce are
those listed in `blob-format.md` §2.5 (plus the `__bytes__` ext wrapper). A
blob that asks for anything else is rejected whole: no partial graph, no
default type, no skipped entry.

### 2.3 Determinism

For a fixed object graph, `reencode` must produce the same bytes on every call
and in every process (see `blob-format.md` §4.4). Cache keys are derived from
those bytes.

### 2.4 Failure logging

A failed revive is logged, and the log line must be safe to ship:

* it may carry the blob's size, the failure class, and the position or ref id
  at which decoding stopped;
* it must **not** contain the blob bytes, a prefix of them, or anything derived
  by rendering them (no `repr`, no hex dump, no base64 of any part of the
  blob).

The same rule holds for exceptions that propagate to the caller: their messages
describe the failure, not the data.

### 2.5 HTTP surface

`revive()` is also exposed over HTTP. The shell and its response shape are
fixed:

| case | status | body |
|------|-------:|------|
| success | 200 | `{"ok": true, "value": <restored object>, "key": "<cache key>"}` |
| failure | 400 | `{"ok": false, "error": "<message>"}` |

Nothing else changes about the envelope when the internals are corrected.

## 3. Tables

### 3.1 Type table loading

The type table is loaded in full before the chunk stream is decoded. Every name
in it is checked against the allowlist of `blob-format.md` §2.5, not only the
names the stream happens to reference.

### 3.2 Duplicate type ids

A type id that appears twice in the type table is an error (`blob-format.md`
§4.1). The service reports it like any other malformed blob. It does not keep
the later entry, the first entry, or merge the two.

### 3.3 Unknown type ids

A `TYPE` value whose id is not present in the table is an error. It is not
mapped to a default container type.

## 4. Restore pipeline

The order of operations is part of the contract:

1. **Header** — validate magic, version, and both offsets before touching any
   other byte (`blob-format.md` §2).
2. **Type table** — load and validate it (`§4.1`, `service.md` §3).
3. **Ref table** — load it; it gives `REF` ids their meaning (`§4.2`).
4. **Chunk stream** — decode records in order, charging declared lengths
   against the memory budget before reading them (`§3.2`, `service.md` §2.1).
5. **Refs** — a `REF` only resolves to a record already decoded; each record is
   decoded at most once and its result reused (`§3.1`, `§3.4`).
6. **Depth** — the structural nesting limit applies while the graph is being
   built, not only at the root (`§3.5`).
7. **Re-encode** — canonical bytes are produced from the finished graph
   (`§4.4`).

A failure at any step aborts the whole revive. Logging follows §2.4.
