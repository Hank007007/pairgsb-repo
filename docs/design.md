# Design notes

This document explains how the SDK is put together and which properties the
design is meant to preserve. The normative rules live in
[`merge-rules.md`](merge-rules.md); where the two disagree, that document wins.

## 1. Shape of the package

```
src/index.js     public surface: load, applyPatch, deepMerge, envOverlay, CODES
src/loader.js    layer orchestration, layer file reading, --set handling
src/merge.js     recursive object merge
src/env.js       environment variable overlay
src/paths.js     dotted path helpers
src/patch.js     JSON Patch engine
src/errors.js    error codes and the error record factory
src/util.js      structural helpers (plain-object test, clone, equality)
```

The modules are plain ES modules with no dependencies. Anything that needs the
filesystem or `process` lives in `loader.js`; the rest is pure and can be
tested in isolation.

## 2. Data flow of `load()`

```
defaults ──clone──► document
                     │
  layers[0]  ──read──┼──merge──►
  layers[1..] ───────┘
                     │
  env                └──envOverlay──►
                     │
  argv               └──flags───────►
                     │
                     ▼
              { config, errors }
```

Two properties matter here:

1. **The document is owned by the caller.** `load()` starts from a clone of
   `defaults` and merges clones of the parsed files. Nothing reachable from
   the arguments is mutated or aliased.
2. **Errors accumulate.** A rejected entry is recorded and the orchestration
   continues with the next entry, so a single run reports everything that was
   wrong with the input rather than stopping at the first problem.

The error array is part of the return value, not a thrown exception. Service
startup code is expected to log both fields; throwing would make it impossible
to report more than one problem per run and would hide which layer was at
fault.

## 3. Why merging is recursive but shallow about types

Layers express partial overrides: an environment file usually names two or
three leaves, not a whole subtree. Recursive merging is what lets an overlay
say `{ "db": { "pool": { "size": 16 } } }` without erasing `db.pool.timeoutMs`.

Everything that is not a plain object replaces. That includes arrays. Element
-wise array merging is ambiguous — there is no way to say "drop the second
element" — so the layering surface deliberately has no opinion beyond
"replace", and the JSON Patch surface carries the explicit operations
(`add` with `-`, `remove`) for callers that need finer control.

A document whose own prototype is not `Object.prototype` would break the
"plain object" test for every reader, and a container reached through the
prototype chain is not part of the document. The merge therefore treats the
document as a tree that must stay finite and rooted: it never reads or writes
through a prototype, and a key that would traverse the chain is not a key of
the document at all.

## 4. Key handling

Every entry point that can introduce a key — layer files, environment
variables, `--set`, JSON Patch — funnels into the same notion of an
admissible key (see `merge-rules.md` §2 and §3). Keeping the notion in one
place is what makes the surfaces agree; a surface that skips the check is
inconsistent with the others regardless of how it behaves on ordinary input.

Three consequences are worth calling out:

- **Absence is not the same as `undefined`.** A key that is present with the
  value `undefined` still counts as present in most readers' eyes; paths are
  resolved with own-property checks so that inherited names never look like
  configuration.
- **Rejection is reported per key.** A layer that is 95% valid still applies
  its 95%; only the offending entry is dropped.
- **Names are not rewritten.** Case-insensitive matching is a property of
  environment variables (§4.1) and of nothing else.

## 5. Type coercion for string-shaped inputs

Environment variables and `--set` values arrive as text. The document is the
authority on type: if the target already holds a number, the incoming text is
parsed as a number; if it holds a boolean, only `true`/`false` spellings are
accepted; if it holds an array, the text becomes one more element coerced the
way the array's elements are. When the target is absent there is no type to
match, so the text is stored as text.

This is intentionally *not* schema driven. The SDK has no schema; the defaults
object is the schema. That keeps the SDK usable without a second declaration,
at the cost of accepting any type the defaults happen to hold.

Coercion failure is an error entry, not a silent fallback to the raw string:
silently storing `"maybe"` where `true`/`false` is expected moves the failure
to a place far from its cause.

## 6. JSON Patch

`applyPatch()` is the hot-reload surface. It works on a clone, applies the
operations in order, and returns the new document alongside the errors. A
failed operation leaves the document as it was before that operation and the
remaining operations are still attempted: a control plane that ships a patch
with one bad pointer should not also lose the ten good operations in it.

Pointer parsing and containment are treated as one concern. A pointer is not
"just a path" — it is a path plus an assertion that the path stays inside the
document. Segments that are empty, `.`, `..`, reserved, or non-canonical array
indices are all the same category of problem: the caller asked for a location
the document does not have.

The engine supports the six operations of RFC 6902 that a configuration
document needs. `test` is included because a hot reload without it cannot
express "only apply this if nobody has changed the value since I read it".

## 7. Testing strategy

- `test/baseline.test.js` pins the ordinary behaviour of every surface: layer
  ordering, array replacement, environment nesting, `--set` precedence,
  purity, and the basic patch operations.
- Additional suites are separate files so that the baseline stays a stable
  reference.
- `examples/load-demo.js` is a smoke test that exercises all four layers with
  a normal configuration, and `examples/proto-check.js` loads a fixture whose
  keys are hostile and reports whether the global object stayed clean.

Tests use `node:test` and `node:assert/strict`; there is no test framework
dependency and no build step.
