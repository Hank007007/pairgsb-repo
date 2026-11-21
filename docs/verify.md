# Verification

`subsetctl verify PKG.json --chain audit.log [--key KEY]` runs offline on
the auditor's machine. It reads nothing except the package, the chain file
and the key supplied on the command line, and prints one line per check.

## §1 Invocation

```
subsetctl verify pkg.json --chain audit.log --key auditor-key
```

* `PKG.json` — the disclosure package from `extract`.
* `--chain` — the main chain file the commitment must be found in. Omitting
  it is only meaningful for a package with no commitment; the chain-presence
  check then fails.
* `--key` — the operator's signing key. Omitting it skips only the signature
  check and is reported as such; every other check still runs.

## §2 Output

One line per check, in the order below:

```
pass  bucket-root       recomputed root over 3 leaf/leaves
fail  leaf-inclusion    leaf seq=4 does not fold to bucket_root  seqs=4
```

The first column is `pass` or `fail`; the second is the check name; the
remainder is a detail string. A check that concerns particular records
appends `seqs=<comma separated seq numbers>`.

All checks always run. The report lists every failing check rather than
stopping at the first one, so a tampered package shows all of what is
wrong.

## §3 Checks

| # | name | passes when |
|---|---|---|
| 1 | `bucket-root` | every leaf digest, recomputed from its `record`, folds along its own proof to `bucket_root` |
| 2 | `leaf-inclusion` | per-record form of check 1: names each `seq` whose proof does not fold |
| 3 | `leaf-actor` | every leaf's `record.actor` equals `package.actor` |
| 4 | `leaf-order` | `leaves` is strictly ascending by `seq`, with no duplicates |
| 5 | `leaf-count` | `len(leaves)` equals `package.count`; reported separately from the root checks |
| 6 | `directory-proof` | the directory leaf over `(actor, bucket_root, count)` folds by side and index to `directory.period_root`, with the proof length implied by `n_buckets` |
| 7 | `commitment-binding` | `commitment.period_id` equals `period_id`; `commitment.period_root` equals `directory.period_root`; the actor's entry in `commitment.buckets` carries the same `root` and `count`; every disclosed `seq` lies in `[from_seq, to_seq]`; `count <= total` |
| 8 | `chain-link` | the chain file parses and its own `prev`/`hash` links verify end to end |
| 9 | `chain-commitment` | the chain contains a `commit` record whose `period_id` equals the package's and whose signed fields equal the package's commitment |
| 10 | `signature` | the commitment's `signature` verifies over its signed fields under `--key` |

Checks 1–4 are per-record. When a record fails check 1 or 2, the report
must name its `seq`; a failing record is never skipped or treated as
absent. A skipped record would let the extractor hide an event and still
claim a pass.

Check 5 is deliberately separate from check 1. A package whose `count`
disagrees with its leaves is wrong even if every supplied leaf is
internally consistent, because the count is what the directory leaf and
the commitment bind.

Check 9 is what ties the package to the ledger. The commitment inside the
package is only a copy; the chain is the authority. A package whose
commitment is missing from the chain, differs from the chain's record for
that period, or would have the chain's record at a different position
fails.

## §4 Exit codes

| code | meaning |
|---|---|
| 0 | every check passed |
| 1 | at least one check failed |
| 2 | usage error: unreadable package or chain, malformed JSON, missing arguments |

Exit code 1 is returned for any failed check, including a signature
mismatch. A signature problem is never downgraded to a warning: the
process must not exit 0 when the commitment is not attributable to the
operator.

## §5 What a passing report means

A pass means: the disclosed records are exactly the actor's complete bucket
for the period, in the committed order; the bucket is the one sealed in the
period root; the period root is the one in the chain for that period; and
the operator signed that chain record. It does not mean the ledger itself
is honest — the operator could have committed a false record — only that
what the auditor holds matches what was committed.

## §6 Reproducing a failure

Every failure detail should be sufficient to reproduce it locally:

* for a bad leaf, the `seq` and the expected versus computed parent;
* for a count mismatch, both numbers;
* for a chain problem, the `period_id` and the chain position where the
  record was expected;
* for a signature problem, which signed field differs first, when the
  package's copy of the commitment differs from the chain's.
