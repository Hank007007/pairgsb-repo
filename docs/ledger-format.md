# Ledger format

This document is normative. It defines the on-disk shape of a ledger and the
guarantees `scanctl verify` is required to check.

## §1. Append-only history

The ledger is a single file holding one JSON object per line (JSONL). It is
append-only: a record, once written, is never modified or removed. Corrections
are expressed by appending new records. Any tool that rewrites, reorders or
truncates the ledger breaks the guarantees in this document.

## §2. Record fields

Every record carries the following fields:

| Field | Type | Meaning |
|---|---|---|
| `seq` | integer | 1-based position in the history |
| `kind` | string | `observation`, `resolved`, `reopened` or `waiver` |
| `fingerprint` | string | identity of the finding, see `fingerprint.md` |
| `findingId` | string | short identifier derived from the fingerprint |
| `ruleId` | string | scanner rule that produced the finding |
| `file` | string | file the finding was seen in |
| `line` | integer | line the finding was seen at |
| `status` | string | resulting state, see `lifecycle.md` |
| `waiver` | object | present on `waiver` records, see `waivers.md` |
| `at` | timestamp | RFC 3339, UTC |
| `prev` | string | hash of the preceding record |
| `hash` | string | hash of this record |

Fields that do not apply to a record are omitted or empty; they are still part
of the hash input described in §4.

## §3. Sequence numbers

`seq` starts at 1 and is strictly increasing. Every append uses the next value
after the highest `seq` already present in the ledger — not the number of lines
in the file, which can differ after a truncated or hand-edited history.

Two records must never share a `seq`, including when two processes append at
the same time. Concurrent appends must be serialized or must use a persistent
counter that cannot hand out the same value twice. A gap is allowed only when a
record was removed, and a removal is itself a violation.

## §4. Hash chain

The first record's `prev` is the empty string. Every other record's `prev` is
the `hash` of the record immediately before it.

`hash` is the lowercase hex SHA-256 of the record's canonical encoding: a compact
JSON object containing the fields of §2 in the order listed above, with `hash`
set to the empty string. A verifier recomputes this value from the stored fields
and compares it with the stored `hash`.

Because `prev` names the predecessor's hash, changing any field of any record
invalidates that record's hash and every `prev` that follows. Editing a record
in the middle of the ledger therefore breaks the chain at the following record;
removing a record breaks it either at the following record's `prev` or at the
sequence number.

## §5. Checkpoints

Every `N` records (default 16) the writer appends a checkpoint to the sidecar
file `<ledger>.checkpoints`, one JSON object per line:

| Field | Meaning |
|---|---|
| `seq` | sequence number of the last record covered |
| `lastHash` | hash of that record |
| `at` | when the checkpoint was taken, RFC 3339 UTC |
| `signature` | MAC over the checkpoint |

The signature is lowercase hex HMAC-SHA256, keyed with the ledger key, over the
UTF-8 string `checkpoint|<seq>|<lastHash>`.

A checkpoint is valid when its signature verifies **and** `lastHash` equals the
hash of the ledger record at `seq`. Both conditions are required; a checkpoint
whose signature was never checked proves nothing.

## §6. Verification

`scanctl verify` recomputes the ledger from the start and must report a failure
when any of the following holds:

1. a record's stored `hash` does not match its contents;
2. a record's `prev` does not match the preceding record's `hash`;
3. a `seq` is not strictly greater than the one before it;
4. a checkpoint's signature does not verify;
5. a checkpoint's `lastHash` does not match the record at its `seq`;
6. a checkpoint refers to a `seq` beyond the end of the ledger.

On failure the command reports the **first** sequence number that fails, and
exits non-zero. Checkpoint failures are reported separately from record
failures, naming the checkpoint's `seq`, so an operator can tell a tampered
record apart from a tampered anchor.

An empty or missing ledger is valid and verifies with zero records.
