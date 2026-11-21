# Design

This document states the intended behaviour of beaconproof. It is a design
statement, not an implementation guide: it says *what* must be true, and leaves
*how* to the code. The normative rules live in `docs/chain-format.md`,
`docs/anchoring.md`, and `docs/receipts.md`; where this document and those
disagree, those win.

## Goals

beaconproof makes a local audit log tamper-evident in two directions at once:

- **inward**: every record commits to its predecessor, so editing, deleting or
  reordering records breaks the chain;
- **outward**: intervals of the chain are handed to an external timestamp
  authority, so the existence of records at a point in time is attested by
  someone other than the local process.

The threat model is a local process (or operator) that can rewrite
`audit.log` and `receipts/` after the fact. It cannot forge the authority's
signature. It can, however, try to:

- edit a record and recompute the chain;
- move a genuine receipt to a different interval;
- delete a receipt that makes an inconvenient interval evident;
- claim evidence for records the authority never saw;
- exploit clock adjustments to confuse range lookups.

Verification must defeat all of these, and must say *which receipt and which
field* is wrong when it does not.

## Components

```
cmd/beaconctl      command line: append / anchor / verify / authority
internal/chain     records, append, replay verification, seq-keyed lookup
internal/merkle    leaf/node hashing and range roots
internal/anchor    policy, submission, catch-up
internal/receipt   receipt storage and verification
internal/tsa       HTTP client and an in-process authority
```

## Hash chain

A record's canonical form is fixed by `docs/chain-format.md` §2. The design
requirements that follow from it:

- **Order is defined by `seq`.** Timestamps are evidence, not an index. Every
  operation that selects records ("the record at time T", "the interval
  `[a, b]`") must be expressed on `seq`. A time-based lookup is only correct if
  it first proves the timestamps are ordered; they are not, because the wall
  clock can step backwards.
- **Timestamps must be distinguishable.** The on-disk format must carry
  sub-second precision, and two records in the same second must hash
  differently. A format that truncates to whole seconds makes distinct events
  collide in the hash input, which turns an append into a rewrite.
- **A backward clock step is legal.** It must not make verification fail and
  must not make lookups return a range that does not exist. The natural
  consequence is that replay validation checks link integrity and `seq`
  density, while range selection ignores `ts` for correctness.

## Merkle ranges

- Leaves are the record hashes in **ascending `seq` order**. Order is part of
  the commitment: permuting leaves must change the root.
- The tree is built pairwise, promoting an odd node by pairing it with itself,
  as described in `internal/merkle`. Domain separation (`0x00` leaf prefix,
  `0x01` node prefix) prevents second-preimage confusion between a leaf and an
  inner node.
- `[from, to]` must be a well-formed interval fully covered by local records.
  Out-of-range bounds, inverted bounds, and intervals with no records are
  errors. An error must be distinguishable from "the root happens to be zero
  bytes"; the empty interval has no root at all, and no receipt may be issued
  for it.

## Anchoring

The policy and interval rules are in `docs/anchoring.md`. The design
constraints:

- **The interval is decided before the request leaves.** `[from, to]` and the
  root are computed together from a snapshot; the request, the hash and the
  stored receipt all refer to that one snapshot. Records appended while the
  request is in flight belong to a later interval.
- **The anchored position advances only on durable success.** A receipt file
  on disk is the commit point.
- **Retry preserves the interval.** A failed attempt does not consume the
  interval; the next attempt asks for the same `[from, to]`.
- **Volume and time are independent triggers.** A quiet chain still anchors
  once `MaxAge` elapses; a busy chain anchors every `N` records. Neither
  substitutes for the other, so a very large `N` cannot suppress anchoring
  under a time policy.
- **Catch-up is a sequence of adjacent chunks.** After an outage the backlog is
  cut into consecutive chunks of at most `N`; the union is exactly the pending
  range, each chunk has one receipt, and no two chunks overlap.

## Receipt verification

The rules are in `docs/receipts.md`. Two of them deserve emphasis because they
are the difference between "a signature exists" and "this chain is evidenced":

- **Root binding.** The signed root must equal the root recomputed from the
  local records in the receipt's interval. A signature over the wrong root is
  worthless.
- **Coverage.** `genTime` must sit in the interval's time window; intervals
  must be contiguous and non-overlapping; and the newest receipt must reach the
  chain head. Each of these turns a specific tampering move into a named
  finding: a stale receipt, a deleted receipt, an overlapping forged receipt, a
  truncated tail.

## Output contract

`beaconctl verify` replays the chain and checks every receipt, then prints one
line per finding, naming the receipt index, the interval, and the field.
Success is printed only when there are no findings — including no unanchored
tail. Partial evidence is reported as failure, not as a warning.
