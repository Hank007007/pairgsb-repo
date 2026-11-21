# Disclosure packages

This document defines the package produced by `subsetctl extract`, the
fields that must take part in verification, and the privacy boundary of a
disclosure.

## §1 Purpose

An operator wants to let an external auditor inspect one account's activity
without handing over the whole ledger. A disclosure package proves:

* *inclusion* — the listed records are in the ledger, in the committed
  order, with the committed count;
* *commitment* — the bucket holding them was sealed into the main chain for
  that period;
* *non-selection* — the package contains the actor's complete bucket for the
  period, so the operator cannot leave out an inconvenient record.

## §2 Package format

The package is a single JSON object with these members.

| field | meaning |
|---|---|
| `version` | package format version |
| `actor` | the actor whose bucket is disclosed |
| `period_id` | identifier of the sealed period |
| `window` | `{since, until}` as supplied to extraction |
| `bucket_root` | root of the actor's bucket |
| `count` | number of records in the bucket |
| `leaves` | one entry per record, see §2.1 |
| `directory` | the bucket's path in the period directory, see §2.2 |
| `commitment` | the chain record that sealed the period, see §2.3 |

### §2.1 Leaf entries

Each entry of `leaves` carries:

* `seq` — the record's sequence number;
* `record` — the full event record as stored in the ledger;
* `proof` — the inclusion path inside the bucket: an ordered list of
  `{side, hash}` entries, `side` being `left` or `right`.

The leaves appear in ascending sequence order. The proof for leaf *i* is
relative to the tree built from all `count` leaves in that order.

### §2.2 Directory member

| field | meaning |
|---|---|
| `actor` | actor name committed in the directory leaf |
| `index` | leaf position in the directory tree |
| `n_buckets` | number of real buckets in the period |
| `path` | ordered `{side, hash}` entries up to the period root |
| `period_root` | root of the period directory tree |

### §2.3 Commitment member

The commitment is a verbatim copy of the `commit` record from the main
chain:

| field | meaning |
|---|---|
| `period_id` | sealed period |
| `period_root` | directory root that was sealed |
| `buckets` | directory entries: `{actor, index, root, count}` |
| `from_seq` | lowest sequence number covered |
| `to_seq` | highest sequence number covered |
| `total` | number of event records in the period across all actors |
| `signature` | operator's signature over §2.3.1 |

#### §2.3.1 Signed bytes

The signature covers a canonical serialization of `period_id`,
`period_root`, `buckets`, `from_seq`, `to_seq` and `total`. It does not cover
the chain link fields or the signature itself. The canonical serialization
must be independent of key order and of the writer's runtime.

## §3 Time window

Extraction selects events in the half-open interval `[since, until)`.

* A record whose timestamp is exactly `until` belongs to the next period,
  not this one. Adjacent periods must never both contain the same record.
* A record whose timestamp is exactly `since` belongs to this period.
* Timestamps must be compared as instants. The same instant written with a
  `Z` suffix, with an explicit numeric offset such as `+08:00`, or with no
  suffix under a declared zone convention must select the same records.
  A timestamp that parses but uses a different notation must not be
  dropped silently.
* A record whose timestamp cannot be interpreted is an error; dropping it
  would make the disclosed bucket disagree with the committed count.

## §4 Fields that must take part in verification

A verifier must not take any of the following on trust. Each must be
recomputed from, or checked against, other material in the package or the
chain:

1. **Leaf digests.** Recomputed from `record` using the canonical leaf
   encoding (`bucket-tree.md` §2). The record's actor must equal the
   package's `actor`.
2. **Leaf ordering and count.** `leaves` must have exactly `count` entries,
   in strictly ascending `seq` order. `count` must agree with
   `len(leaves)`, with the directory leaf, and with the bucket entry in the
   commitment.
3. **Inclusion proofs.** Every leaf's proof must fold to `bucket_root`.
   A proof that does not fold is a failed record, not a record to skip; the
   check must name the offending `seq`.
4. **Bucket root.** The root recomputed from the leaves must equal
   `bucket_root`.
5. **Directory proof.** The leaf made from `actor`, `bucket_root` and
   `count` must fold, by side and position, to `directory.period_root`; the
   proof must have the length implied by `n_buckets`; padding slots are not
   provable.
6. **Commitment binding.** `commitment.period_id` must equal
   `package.period_id`; `commitment.period_root` must equal
   `directory.period_root`; the actor's entry in `commitment.buckets` must
   carry the same `root` and `count`; `actor` must equal `package.actor`.
   `count` must not exceed `commitment.total`, and the period's
   `from_seq`/`to_seq` must bracket every disclosed `seq`.
7. **Chain presence.** The commitment supplied in the package must be
   located in the chain supplied with `--chain`: a record with the same
   `period_id`, the same signed fields, and the same chain position must
   exist there. The chain's own links must verify. A commitment that is
   absent from the chain, or that differs from the chain's record for that
   period, fails.
8. **Signature.** The operator's signature over the commitment's signed
   fields (§2.3.1) must verify under the auditor's configured key.

## §5 Verification outcomes

Verification is a set of named checks. Every check produces a status and
a human-readable detail; where a check is per-record, it names the `seq`.
Verification passes only if every check passes. See `verify.md` for the
check list, the ordering of reported problems, and exit codes.

## §6 Privacy boundary

A package discloses:

* the full records of the target actor for the period,
* the target actor's bucket shape (its root and count),
* the directory path: sibling digests and the period root.

A package must not disclose:

* other actors' records,
* other actors' bucket roots or counts,
* other actors' identity beyond what the operator already knows.

These hold because sibling digests are one-way: they commit to other
buckets without revealing them. The bucket's own records are not hidden —
selective disclosure hides *which records exist*, not *what the disclosed
records say*.

The auditor must also consider that:

* the period root is public, so the operator can later prove that a
  particular other-actor bucket existed, but cannot open it without that
  actor's records;
* the `total` field reveals how many events occurred in the period across
  all actors; that aggregate leak is intentional and is what lets the
  auditor see that the disclosed bucket is not the whole period;
* the signature binds the operator to the period root, so a period cannot
  be re-sealed with different contents under the same identity without
  detection.

## §7 Threat model for the auditor

The auditor must assume the extractor is adversarial. It may:

* omit records from `leaves`, or reorder them;
* alter a record's contents while keeping its `seq`;
* present another actor's bucket as the target's;
* fabricate, truncate, reorder or re-orient proofs;
* substitute an old commitment, or a commitment for a different period;
* reuse padding slots as if they held data;
* keep an extracted package and later claim a different one was issued.

The checks in §4 are exactly the ones that must defeat these attempts. An
implementation that skips a record whose proof fails, or that reads the
commitment from the package without consulting the chain, defeats none of
them.

## §8 Out of scope

* Confidentiality of the ledger at rest.
* Availability of the ledger.
* Key management for the operator's signing key.
* Aggregation across periods; a package covers one period.
