# protolens schema registry

This document describes the schema model and the registry that serves it. It
states required behaviour only.

## 1. Schema model

A schema is a named **type**. Each type has one or more **versions**. A version
carries an ordered list of **fields**; each field declares:

- `number` — the wire field number (1 .. 2^29-1, see `wire.md` §2.3);
- `name` — the key used in decoded objects;
- `type` — one of the scalar or message types in `wire.md` §3;
- `repeated` — whether the field collects multiple values into an array;
- `packed` — preferred output form for repeated scalars (`wire.md` §5.1);
- `messageType` — for `message` fields, the name of the nested type.

A version is immutable once published. Field numbers, names and types are all
part of a published version and do not change underneath it.

## 2. Versions and hot reload

### 2.1 Version numbers

Versions are monotonically increasing integers scoped to a type. Version `1` is
the first published version. Senders name the version they encoded against, and
a decoder must interpret the bytes using that version's field numbers.

### 2.2 Hot reload

The registry supports hot reload: a new snapshot of the schema set is published
while the process is serving traffic.

- Reload **bumps the version** of any type whose definition changed. A type
  whose definition is unchanged keeps its version. A changed definition is
  never published under an existing version number.
- Reload swaps the registry's notion of "current" atomically. A reader either
  sees the old snapshot or the new one, never a mixture.
- A request that is **already being processed** continues to use the version it
  resolved when it started (§2.3). Reload affects only requests admitted after
  the swap.
- Reload must not mutate any previously published version object. An in-flight
  request holding a reference to `(T, v1)` must still observe exactly the
  fields of `(T, v1)` after the reload, including when a new version moved,
  added or removed field numbers.

### 2.3 Resolution and in-flight state

- A request resolves `(type, version)` to an immutable schema snapshot **once**
  at admission (`gateway.md` §1, step 2) and uses that snapshot for its whole
  lifetime. It must not consult the registry's mutable "current" pointer again
  after admission.
- If the request names a version explicitly, that version is resolved exactly.
  If it omits the version, the version current at admission is used.
- Resolving a `(type, version)` that was never published is an error; it is not
  silently rounded to the current version.

## 3. Caching

### 3.1 Key

- Resolved schemas are cached under the composite key **(type, version)**.
  The key is exactly the pair; caching on the type name alone is not sufficient
  because two versions of one type may assign the same field number to
  different fields.
- Any in-process cache keyed by type name alone does not distinguish two
  versions of a type and therefore cannot satisfy this rule: after a reload
  bumps a version, a request for the new version must resolve the new fields,
  while an in-flight request for the old version must keep resolving the old
  fields.
- The key format is defined once, in `cacheKey`. Every cache in this layer is
  expected to use it rather than reconstructing the key inline.

### 3.2 Field lookup

- Looking up a field by number is scoped to one version. Field number `3` in
  `(T, v1)` and field number `3` in `(T, v2)` are unrelated lookups and may
  return different fields.
- A field number not present in the resolved version is unknown to that version
  and is handled as an unknown field (`wire.md` §4.2), not as an error.

### 3.3 Invalidation

- Reload publishes new `(type, version)` entries. Entries for superseded
  versions are retained for as long as any in-flight request may reference
  them, and may be evicted once unreferenced. Eviction is invisible to
  correctness.

## 4. Errors

Resolution failures report a distinct machine-readable code and message. They
are distinguishable from wire-format failures (`wire.md` §4.3) and from
signature failures (`gateway.md` §3.4).
