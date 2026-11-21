# Anchoring

This document is normative. It defines when the library requests a receipt,
how a pending backlog is caught up after the authority was unreachable, and how
intervals are cut.

## §1 Terms

- **Anchored position** (`anchoredSeq`): the largest `toSeq` of any receipt
  that has been durably stored. It is recovered from the receipt directory on
  startup, not from memory.
- **Pending**: records with `seq > anchoredSeq`.
- **Interval**: a closed range `[fromSeq, toSeq]` of pending records submitted
  as one timestamp request.
- **Generation time**: the `genTime` the authority returns for an accepted
  request.

## §2 When to anchor

A new interval MUST be submitted when **either** condition holds:

1. **Volume**: `pending >= N`, where `N` is the configured interval size; or
2. **Time**: the wall-clock duration since the last successful anchor is at
   least `MaxAge`, and `pending >= 1`.

Condition 2 exists so that a quiet chain still gets evidence, and so that a
large `N` cannot starve anchoring indefinitely. The two conditions are
independent: neither may be subsumed by the other. When `MaxAge` is zero the
time condition is disabled and only volume applies.

An anchor MUST NOT be attempted when `pending == 0`.

## §3 Interval boundaries

- The interval to anchor is exactly `[anchoredSeq + 1, headSeq]` at the moment
  the policy fires, where `headSeq` is the newest record at that moment.
- The interval's `toSeq` MUST be captured **before** the network request is
  built and MUST NOT change afterwards. It is the boundary that is sent to the
  authority, hashed, and stored with the receipt.
- The Merkle root MUST be computed over exactly the records in the interval
  that was sent. If records are appended while the request is in flight, those
  records belong to the *next* interval, not the current one.
- The `fromSeq` of the next interval MUST be `toSeq + 1` of the previous one.
  Intervals MUST NOT overlap.

## §4 Failure and retry

- A failed request (timeout, non-200, malformed token, invalid signature) MUST
  leave `anchoredSeq` unchanged.
- A failure MUST NOT be recorded as a success, but it also MUST NOT cause an
  interval to be skipped. The next attempt MUST use the same `fromSeq`.
- Retrying MUST be idempotent from the chain's point of view: at most one
  receipt may exist for a given `[fromSeq, toSeq]`.
- A partially written receipt (request sent, response lost) is not a receipt;
  the interval remains pending and is retried.

## §5 Catch-up after an outage

After a period in which the authority was unreachable:

1. The pending backlog is anchored in **consecutive, non-overlapping**
   intervals of at most `N` records each.
2. Every pending record MUST end up inside exactly one interval. Nothing is
   skipped, nothing is covered twice.
3. Exactly one receipt is stored per interval.
4. A backlog larger than `N` MUST be split. It MUST NOT be collapsed into a
   single interval if doing so would exceed the configured interval size.

For a backlog `anchoredSeq+1 .. headSeq`, the intervals are:

```
[anchoredSeq+1, min(anchoredSeq+N, headSeq)]
[anchoredSeq+N+1, min(anchoredSeq+2N, headSeq)]
...
```

## §6 Relationship to verification

`beaconctl verify` MUST confirm two coverage properties in addition to the
per-receipt checks in `docs/receipts.md`:

- **No gaps**: for consecutive receipts ordered by `fromSeq`, the second
  `fromSeq` is the first `toSeq + 1`. A gap is reported with the missing
  `[fromSeq, toSeq]`.
- **No unanchored tail**: the newest receipt's `toSeq` equals the chain head
  (or the number of records after the last receipt's `toSeq` is zero). If
  records exist beyond the last receipt, `verify` MUST NOT report success; it
  MUST state how many records are unanchored and the interval that covers them.
