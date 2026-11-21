# Diff semantics

This document is normative. It defines what `scanctl diff` reports and how a CI
job may rely on it.

## §1. Inputs

- the **current scan**: the findings the scanner produced for this build;
- the **ledger**: the history the repository has accumulated so far;
- the **baseline** (optional): a scan file that this branch is allowed to
  contain, typically the accepted state of the main branch.

## §2. Identity

Findings are compared by fingerprint (see `fingerprint.md`), never by file,
line or message. Each fingerprint appears at most once in the output, even if
the scanner reports it more than once in the same scan.

## §3. What is reported as new

A fingerprint from the current scan is reported as new when it is absent from
the baseline and its effective state at the reference time is not `waived`.

Two consequences matter for CI:

- a finding that is in the baseline stays accepted and is not reported again,
  even if it has been there for a long time;
- a finding whose waiver has expired is no longer waived. If it is absent from
  the baseline it is reported again, at its underlying severity. An expired
  waiver never suppresses a finding.

A finding covered by an active waiver is not new even when it is absent from the
baseline.

## §4. What is reported as resolved

A fingerprint present in the baseline and absent from the current scan is
**resolved**. Resolving is a state change, not merely an omission from the
output:

- the fingerprint appears in the diff output under "resolved";
- the ledger receives a `resolved` record for it, moving it to `fixed`;
- if the fingerprint is not known to the ledger at all, no record is written,
  because there is no history to update.

A resolved finding must not be dropped silently: omitting it from both the
output and the ledger would make the history claim the finding still exists.

## §5. The exit code

`diff` exits with status 1 when the run contains at least one **new high-risk
finding**, where high-risk means severity `high` or `critical`. It exits 0
otherwise. Findings that are `fixed`, `waived` or `resolved` at the time of the
run never count towards that number, regardless of their severity field.

## §6. Determinism

The same inputs — the same scan, the same baseline, the same ledger, and the
same reference time — must produce byte-identical output. CI compares the JSON
form against a checked-in snapshot, so the order of entries is part of the
contract:

- `new` entries are ordered by fingerprint, then file, then line;
- `resolved` entries follow the same ordering;
- `newHigh` counts only the entries reported as new.

Iteration order of any internal map must not leak into the output. Two
consecutive runs over the same inputs that differ only in map iteration order
are a failure of this contract.

## §7. Reference time

Time-dependent behaviour (waiver expiry) uses a single reference time taken at
the start of the run, so that one invocation cannot see a waiver expire between
two comparisons.
