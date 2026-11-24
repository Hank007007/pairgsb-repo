# Consistency — zookies and snapshots

tupleresolve exposes a very small consistency model: mutations produce
zookies, reads may ask to see at least a given zookie, and decision caches
must respect both.

## 1. Every mutation advances the zookie

The tuple store is versioned with a monotonically increasing counter per
tenant. The counter is the tenant's *data version*.

* A successful **insert** assigns the tuple a version strictly greater than
  every version previously assigned in that tenant, then returns a zookie
  that encodes at least that version.
* A successful **delete** does the same. A delete is a mutation: even though
  it removes data rather than adding it, a reader that has been told
  `at_least = Z` (where `Z` came from the delete) must not be served a
  snapshot in which the tuple still exists.

Deleting a tuple that does not exist is a no-op for data but is still a
successful request; whether it advances the version is unspecified — the
implementation may treat it as a mutation. What is *not* allowed is a delete
that removes a tuple without advancing the version.

Batch writes advance the version once per batch (all tuples in a batch share
the version of the batch's commit point) and return one zookie whose version is
at least the batch's version.

The zookie token is opaque; it may be a string, an object, or a number. It
must round-trip through `at_least` without loss of ordering information.

## 2. `at_least` reads

A check with `at_least = Z` must be evaluated against a snapshot that contains
every mutation completed before `Z` was issued and none that was assigned a
version later than `Z`'s version.

In this implementation the store is in-process and keeps full history, so the
practical rule is:

* resolve `Z` to its numeric version `v`;
* the read must not use a cached decision whose snapshot version is below `v`;
* the tuple set consulted for the read is the set of tuples whose
  `createdVersion <= v` **and** which were not deleted at a version `<= v`.

Equivalently: a tuple deleted with zookie `Z_d` is invisible to any read with
`at_least >= Z_d`, and visible again to a read with `at_least < Z_d` that
predates the delete only if history retention allows it. (Snapshot reads older
than the current version are supported; if the implementation retains only
current state, an `at_least` in the past may be satisfied by current state,
which is still correct because current state is never older than `Z`.)

If the caller passes an `at_least` that cannot be resolved (malformed token,
unknown token), the check fails with `invalid_request`. A token from another
tenant is not resolvable.

Omitting `at_least` means "current state"; the read must still not use a
cached decision that is older than the current namespace configuration.

## 3. Decision cache

Decisions (`check` results) may be cached. A cached decision is only reusable
when **all** of the following are equal to the current request:

1. the tenant;
2. the object;
3. the relation;
4. the subject;
5. the **namespace configuration versions** of every namespace the decision
   depended on — at minimum the namespace of the queried object; if the
   implementation stores it, the full set of namespaces traversed;
6. the effective **zookie floor** of the request: the numeric version implied
   by `at_least`, or, when `at_least` is omitted, the store's current data
   version at the time the decision was computed.

Rule 6 means that a mutation that changes the data must make prior decisions
unusable even when the caller passes no `at_least`: the entry's floor is below
the new current version, so it is stale. (An implementation may instead
invalidate by version, but the observable requirement stands: after writing a
new tuple, the same `(object, relation, subject)` must be recomputed.)

`expand` results are not decisions: they must not be served from or stored in
the decision cache. A cache entry produced by `check` and an entry produced by
`expand` must never be confused with each other.

Errors are not cached.

## 4. What a correct implementation must make observable

* insert → zookie increases; delete → zookie increases;
* a check with the delete's zookie does not see the deleted tuple;
* a check with no zookie after an insert that grants access returns true, even
  if the same check returned false before the insert;
* reloading a namespace with a new version changes evaluation according to the
  new document, even if a compiled plan for the old version is still in memory;
* a check whose result was cached before a namespace reload is not reused
  after the reload.
