# Lifecycle

This document is normative. It defines the states a finding can be in and the
transitions between them.

## §1. States

| State | Meaning |
|---|---|
| `open` | the finding is known and has not been dealt with |
| `waived` | the finding is accepted for a bounded time by an approved waiver |
| `fixed` | the finding was resolved; the scanner no longer reports it |
| `reopened` | a resolved finding was reported again by a later scan |

## §2. The state machine

Only the edges drawn here are legal. Every other edge is rejected: the ledger
does not silently accept a transition that is not part of the lifecycle.

```
   open   --fix----------->  fixed
   open   --waive--------->  waived

   reopened --fix--------->  fixed
   reopened --waive------->  waived

   fixed  --reappear------>  open
   fixed  --reopen-------->  reopened

   waived --fix----------->  fixed
   waived --unwaive------->  open
```

Expressed as a table:

| From | Allowed to |
|---|---|
| `open` | `fixed`, `waived` |
| `reopened` | `fixed`, `waived` |
| `fixed` | `open`, `reopened` |
| `waived` | `open`, `fixed` |

Any pair not in this table — for example `fixed -> waived`, `closed -> fixed`,
or `waived -> reopened` — is invalid. A rejected transition leaves the finding
in its previous state and the caller receives an error.

`closed` is not a ledger state. A record claiming it is malformed and must be
rejected.

## §3. Effective state and waivers

A finding whose recorded state is `waived` is only actually waived while its
waiver is in force. The effective state at time `t` is:

- `waived`, when the recorded state is `waived` and the waiver's `expiresAt` is
  at or after `t`;
- otherwise the recorded state with the waiver disregarded — for an expired
  waiver this means the finding is treated as `open` again.

An expired waiver is not deleted and its record stays in the history. It simply
stops having an effect: the finding reappears in `diff` output and is reported
at its underlying severity. Re-waiving requires a new waiver record, with a new
approval and a new expiry.

Everything the ledger displays or hands to `diff` uses the effective state, not
the raw recorded state.

## §4. Sequence numbers under concurrency

Every append takes the next sequence number after the highest one already
present, and the pair (sequence number, hash chain) must remain unique and
strictly increasing even when more than one process appends to the same ledger
in the same second. Wall-clock time is not a source of uniqueness; two records
written in the same second still get different `seq` values.

## §5. Recording transitions

| Situation | `kind` | Resulting state |
|---|---|---|
| scanner reports a finding not in the ledger | `observation` | `open` |
| scanner reports a finding last recorded as `fixed` | `reopened` | `reopened` |
| scanner no longer reports a finding that was in the baseline | `resolved` | `fixed` |
| an approved waiver is recorded | `waiver` | `waived` |
