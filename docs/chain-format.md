# Local chain format

This document is normative. It specifies how audit records are encoded, how a
record's hash is computed, and what invariants a valid local chain must hold.

## §1 Record fields

Each record is one JSON object on one line of `audit.log`, with these fields:

| field    | type   | meaning                                            |
|----------|--------|----------------------------------------------------|
| `seq`    | int64  | position in the chain, starting at 1, dense        |
| `ts`     | string | when the event was recorded, RFC 3339 with fraction |
| `actor`  | string | who or what caused the event                       |
| `action` | string | short verb naming the event                        |
| `body`   | string | free-form payload                                  |
| `prev`   | string | lower-case hex hash of the previous record          |
| `hash`   | string | lower-case hex hash of this record                 |

`prev` is the empty string for `seq == 1`. Every later record's `prev` MUST
equal the `hash` of the record with `seq-1`.

## §2 Hash input

The record hash is `SHA-256` over a single canonical string. The canonical
string is the following fields, in order, joined by the unit separator byte
`0x1F`:

```
ts, actor, action, body
```

All other fields are the exact UTF-8 bytes stored in the record. `seq`, `prev`
and `hash` itself are not inputs. The digest is rendered as 64 lower-case
hexadecimal characters.

`seq` and `prev` are structural fields: the replay checks that `seq` is dense
and that `prev` names the previous record's hash, so neither can be changed
without a chain finding. The hash commits to the event content and to the time
it was recorded.

Because the hash covers the event, two records must never share a hash. In
particular, two events with the same actor, action and body appended close
together must still hash differently — the recorded time is what separates
them (see §3).

## §3 Timestamps

`ts` MUST be an RFC 3339 timestamp with at least nanosecond precision (for
example `2026-11-21T10:00:00.123456789Z`), always normalised to UTC with a `Z`
offset.

Wall clocks are not monotonic. A clock adjustment, NTP step, or VM migration
may make a later record carry an earlier timestamp than its predecessor. This
is permitted: **`seq`, not `ts`, defines chain order**. Consequently:

- Verification MUST NOT reject a chain merely because `ts` decreases.
- Any lookup or interval selection that is expressed in terms of records MUST
  be keyed by `seq`. A binary search over `ts` is only valid if the
  implementation first establishes that the timestamps are sorted, which clock
  adjustments violate.
- Two records written within the same wall-clock second MUST still be
  distinguishable. Their timestamps MUST differ in the sub-second component,
  and their hashes MUST differ.

Timestamps serve as evidence of *when* something was recorded, not as a
sorting key for correctness.

## §4 Sequence numbers

`seq` values are dense: for a chain of `n` records the values are exactly
`1..n`, each appearing once. A gap, duplicate, or out-of-order value is a
chain defect. The verifier reports the offending record's `seq`.

## §5 Append rules

- Appending MUST assign `seq = previous.seq + 1` (or 1 for an empty chain).
- Appending MUST set `prev` to the previous record's hash.
- The new record MUST be durably written (flushed/synced) before `Append`
  returns successfully.
- The timestamp MUST be captured at append time. The format MUST retain
  sub-second precision.
- Records are never modified or deleted in place.

## §6 Verification

`beaconctl verify` replays the chain from the start and reports, for each
finding, the record `seq` and the field at fault. It MUST detect at minimum:

1. a stored `hash` that does not match the recomputed hash (§2);
2. a `prev` that does not match the previous record's hash (§1);
3. a non-dense or out-of-order `seq` (§4);
4. a `ts` that is not a valid RFC 3339 timestamp (§3).

Chain replay is independent of receipt checking; both run on every `verify`.
