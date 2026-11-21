# Recurrence rules

This document defines how invitefence interprets the recurrence rule supplied
by the meeting front end and how it expands that rule into concrete
occurrences for the busy-time calculation.

## 1. Input

The front end supplies the rule as the value of an `RRULE` property, e.g.

```
FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE;COUNT=10
```

The rule is accompanied by the meeting's time zone (an IANA zone name such as
`Europe/Berlin`) and by the meeting start time.

## 2. Accepted rule parts

The following parts are recognised:

| Part | Values | Notes |
|---|---|---|
| `FREQ` | `DAILY`, `WEEKLY`, `MONTHLY`, `YEARLY` | required, exactly once |
| `INTERVAL` | positive integer | optional, defaults to `1` |
| `COUNT` | positive integer | optional; mutually exclusive with `UNTIL` |
| `UNTIL` | date or date-time | optional; mutually exclusive with `COUNT` |
| `BYDAY` | comma-separated `MO,TU,WE,TH,FR,SA,SU`, optionally prefixed with a positive integer (`1MO`, `-1FR`) | optional |
| `BYMONTHDAY` | comma-separated integers in `1..31` or `-31..-1` | optional |
| `BYMONTH` | comma-separated integers in `1..12` | optional |
| `WKST` | one of the weekday codes | optional |

Rules:

1. `FREQ` is mandatory. A rule without `FREQ`, or with a `FREQ` value that is
   not one of the four accepted values, is rejected.
2. A part may appear at most once. A repeated part is rejected.
3. `COUNT` and `UNTIL` may not both be present. A rule that contains both is
   rejected; neither silently wins.
4. A part whose name is not in the table above (for example `BYSECOND`,
   `BYWEEKNO`, `BYHOUR`, `FREQ=MINUTELY`, `X-...`) is **unknown**. Because the
   clients receiving the invite do not all implement the full grammar, an
   unknown part makes the rule unsupported: the invite is rejected rather
   than generated with a rule whose meaning the service cannot predict.
5. `COUNT` and `INTERVAL` must be positive integers; a non-numeric or
   non-positive value is rejected.
6. `UNTIL` is either a date (`YYYYMMDD`) or a UTC date-time (`YYYYMMDDTHHMMSSZ`).
   A `UNTIL` value carrying a local time (one without the trailing `Z`) is
   converted to UTC using the meeting time zone before comparison. If the time
   zone is unknown or the value cannot be converted unambiguously, the rule is
   rejected rather than compared on the wall-clock string.

### 2.1 Expansion limit

The number of instances the service will ever materialise for a single rule
is bounded. The bound is **1000 instances per request**.

- If the rule would generate more than 1000 occurrences within the window
  (whether because `COUNT` is large or because the window is long), the
  request is rejected with an error. The service does not silently truncate
  the series, and does not send an invite whose recurrence it could not fully
  evaluate.
- The same limit, expressed by the same value, governs both the generated
  `RRULE` and the busy-time calculation. A request the invite path accepts must
  be evaluated for busy time with the same bound, and a request whose busy-time
  evaluation would exceed the bound must be rejected by the invite path as
  well. The two paths may not use different limits.
- Expanding an unbounded rule over a long window must fail fast: the service
  never performs work proportional to an attacker-chosen `COUNT` such as
  `FREQ=MINUTELY;COUNT=100000`. A rule of that shape is rejected in
  milliseconds.

## 3. Expansion

Occurrences are produced for the half-open window `[windowStart, windowEnd)`.

1. The first occurrence is the meeting start. Subsequent occurrences advance
   by `FREQ` × `INTERVAL`, filtered by the `BY*` parts when present.
2. Expansion stops at the first of:
   - an occurrence at or after `windowEnd`;
   - reaching `COUNT` occurrences (when `COUNT` is given);
   - passing the `UNTIL` instant (when `UNTIL` is given), compared as absolute
     instants;
   - reaching the expansion limit in §2.1.
3. An occurrence exactly at `windowEnd` is not included; an occurrence exactly
   at `windowStart` is included.
4. `COUNT` counts occurrences of the series from the meeting start, not
   occurrences inside the window.

## 4. Unknown or ambiguous time zones

- The meeting time zone must be a recognised IANA zone name. An unknown zone
  name is rejected.
- If an occurrence's local time falls in a gap or an overlap created by a DST
  transition, the occurrence is resolved with the zone's standard offset. A
  zone that cannot be resolved at all is rejected.
- If no time zone is supplied, `UTC` is assumed, and the `RRULE`/`UNTIL`
  comparison is done in UTC.

## 5. Date-time forms

- A timed meeting is emitted as `DTSTART;TZID=<zone>:<local time>`.
- `UNTIL` in the emitted `RRULE` is always a UTC instant (`...Z`), so that
  clients in any zone agree on where the series ends.
- A date-only meeting has no time zone and no `TZID` parameter.