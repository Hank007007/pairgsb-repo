# Time: source time, ship time, skew

## §1 Source time

1. Every event carries a **source time** from the source file's `ts` field. It
   is the time the business process observed the event, not the time the relay
   saw it.
2. The source time is transported unchanged. `source_ts` on the shipped record
   must equal the source line's `ts` (normalised to an ISO-8601 UTC string if
   the source used epoch milliseconds).
3. A relay must never replace the source time with its own clock. Doing so
   destroys the ordering and correlation the SIEM relies on.

## §2 Ship time

`ship_ts` is the wall-clock time at which the relay prepared the batch that
contains the record, as an ISO-8601 UTC string.

## §3 Skew

`skew_ms` is `ship_ts - source_ts` in milliseconds, as an integer. It is
computed from the two values actually carried on the record. A negative value
means the source clock is ahead of the relay clock.

## §4 Stability

`source_ts` is part of the ordering key and the digest (`batch-format.md` §3–4);
it is therefore stable across retransmissions. `ship_ts` may differ on a retry
and is not part of the digest.
