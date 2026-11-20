# Storage contract

Two storage implementations are provided: an in-memory store and a file-backed
store. They expose the same interface and must behave identically from the
caller's point of view. This document states the properties both must have.

## Interface

| Operation | Contract |
|---|---|
| `put(token, record)` | Persists `record` so that it can later be found from `token`. Reports success. |
| `findByToken(token)` | Returns a copy of the record associated with `token`, or `null`. |
| `remove(token)` | Removes the record if present. Returns `true` when a record was removed, `false` when none was present. |
| `wasConsumed(token)` | Returns `true` when a record for `token` has previously been removed by `remove`. |
| `pruneExpired(now)` | Removes every record whose expiry is at or before `now`. Returns the number removed. |
| `size()` | Number of live records. |
| `consumedCount()` | Number of consumed entries tracked. |

All operations are asynchronous.

## Indexing

1. Records are indexed by a derived key, not by the plaintext token. The key is
   derived from the token with the service pepper (§2 of the token contract).
2. The plaintext token never appears in any structure the store owns: not in the
   in-memory index, not in any value, not in the `consumed` bookkeeping.
3. Two stores configured with different peppers must not resolve each other's
   keys. A token that was filed under pepper A must not be found by a store
   configured with pepper B.
4. The derived key is deterministic for a given (token, pepper) pair so that a
   fresh store instance reading an existing file can find records written by an
   earlier process.

## The file format

The file is UTF-8 JSON with this shape:

```json
{
  "records": {
    "<derived-key>": {
      "id": "tok_...",
      "subject": "user@example.com",
      "purpose": "password-reset",
      "meta": {},
      "issuedAt": "2026-01-01T00:00:00.000Z",
      "expiresAt": "2026-01-01T00:15:00.000Z",
      "digest": "<digest>"
    }
  },
  "consumed": ["<derived-key>"]
}
```

Requirements on the file:

1. Every object key in `records` and every entry in `consumed` is a derived key.
   Searching the file for the plaintext token, or for a prefix of it, must find
   nothing.
2. The file is written atomically: a reader never observes a partially written
   document, and a failed write leaves the previously committed file intact.
3. Missing or empty files are treated as an empty store. A malformed file is an
   error; it is not silently replaced with an empty store.
4. A store instance that has just been constructed reflects the contents of the
   file on first use. Two instances pointed at the same path in the same process
   are not required to stay in sync with each other; a new instance must always
   observe the committed contents of the file.

## Removal and failure

1. `remove` reports whether a record was present. Callers use this to decide
   whether they won a race, so the return value must be accurate.
2. A failed write during `remove` must reject. It must not resolve `true`, and
   it must not resolve `false` in a way that is indistinguishable from "no such
   record".
3. Nothing in the store removes a record except `remove` and `pruneExpired`.

## Memory store specifics

The memory store has no file. It must still derive keys the same way, so that
the two implementations accept and reject the same tokens for the same pepper.
`put`, `findByToken`, `remove`, `wasConsumed` and `pruneExpired` must honour the
same semantics as the file store. `snapshot()` returns a deep copy of the
internal state for inspection.
