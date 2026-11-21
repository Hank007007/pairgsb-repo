# Receipts

This document is normative. It defines what a receipt contains, which fields a
verifier must check, and how receipts relate to each other.

## §1 Storage

Receipts live in `receipts/`, one JSON object per file. The conventional file
name is `%06d-%06d.json` for `fromSeq` and `toSeq`, but verification MUST use
the fields inside the file, not the name.

## §2 Fields

| field     | type   | meaning                                          |
|-----------|--------|--------------------------------------------------|
| `fromSeq` | int64  | first record in the anchored interval            |
| `toSeq`   | int64  | last record in the anchored interval             |
| `root`    | string | Merkle root of `[fromSeq, toSeq]`, lower-case hex |
| `genTime` | string | RFC 3339 time the authority issued the token     |
| `sig`     | string | base64 authority signature                       |
| `tsa`     | string | identifier of the authority                      |

`fromSeq >= 1` and `toSeq >= fromSeq` MUST hold.

## §3 Root encoding

The Merkle root is always carried as **lower-case hexadecimal**, in the
request, in the token the authority signs, and in the stored receipt. All three
representations MUST be compared as the same byte string. A verifier MUST NOT
compare one encoding against another; it MUST decode both sides to bytes (or
normalise both to the same encoding) before comparing.

## §4 Signature

The signature covers the message `root + "|" + genTime`, where `root` is the
lower-case hex string exactly as stored and `genTime` is the string exactly as
stored. The verifier MUST check the signature with the authority's public key.
A receipt whose signature does not verify is reported against field `sig`.

## §5 Root binding

A signature only proves the authority saw *some* root. Verification MUST also
establish that the root is the one this chain produces:

1. Recompute the Merkle root over the local records `[fromSeq, toSeq]` using
   the rules in `docs/chain-format.md` and the Merkle construction in
   `docs/design.md`.
2. Compare the recomputed root with the receipt's `root`, byte for byte.
3. If they differ, report field `root` with the receipt's 1-based index and
   its `[fromSeq, toSeq]`.

A receipt whose root does not match, or whose interval is not fully covered by
local records, MUST NOT count as evidence.

## §6 Generation time

`genTime` MUST fall inside the time window the interval covers, with a
configurable tolerance (default: 10 minutes). The window is bounded by the
`ts` of record `fromSeq` and the `ts` of record `toSeq`. A receipt issued
before the interval's first record or long after its last record cannot be
evidence that the interval existed at that time, and MUST be reported against
field `genTime`.

Tolerance exists because record timestamps and authority time come from
different clocks. It MUST be small relative to the retention period being
claimed; a receipt from days ago attached to a fresh interval is not covered by
any reasonable tolerance.

## §7 Interval continuity

Receipts are ordered by `fromSeq`. For each consecutive pair `A`, `B`:

- If `B.fromSeq <= A.toSeq`, the intervals **overlap**. Overlap is not
  permitted; the verifier MUST report it, naming both receipts and the
  overlapping range `[B.fromSeq, A.toSeq]`.
- If `B.fromSeq > A.toSeq + 1`, there is a **gap**. The verifier MUST report it
  and name the missing interval `[A.toSeq + 1, B.fromSeq - 1]`.

Removing a receipt in the middle of a run of otherwise consecutive receipts
therefore always produces a reportable gap. Silent acceptance of a gap is a
verification failure.

## §8 Tail coverage

The newest receipt's `toSeq` defines how much of the chain is evidenced. If the
local chain contains records with `seq > newest.toSeq`, verification MUST
report the unanchored tail: the count of such records and the interval
`[newest.toSeq + 1, headSeq]`. `verify` MUST NOT print success while an
unanchored tail exists.

## §9 Reporting

Every finding names:

- the 1-based index of the receipt in verify order (or "chain" for chain
  findings),
- the receipt's `[fromSeq, toSeq]`,
- the field at fault (`root`, `genTime`, `sig`, `range`, or `interval`),
- a human-readable detail.

`verify` exits non-zero if any finding was reported.
