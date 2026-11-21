# Retention

Retention deletes blocks that have aged out of the retention window. Because
sealed blocks are write-once, deletion is the only way an archive shrinks, and
it is the only operation that intentionally removes evidence. The rules below
say exactly how that must happen and how the resulting archive still verifies.

## 1. Expiry

- The retention window is given as a duration (`30d`, `12h`, `90m`, `4w`).
- **A block expires based on the time it was sealed**, recorded in its
  sidecar (`sealed_at`), not based on any filesystem metadata. File
  modification times are mutable by any local process (`touch`), and must
  never influence the decision to delete archival evidence.
- If the sidecar is missing or has no usable `sealed_at`, the block is not
  eligible for retention deletion; it is reported instead.
- Only sealed blocks are eligible. The active log file is never deleted by
  retention.
- `--dry-run` reports what would be removed and changes nothing.

## 2. What deletion must leave behind

Deleting a block removes the segment file and its sidecar. It must also leave
a durable record of the deletion — a **tombstone** — in the archive:

- which block number was removed;
- when, and under which retention window;
- the block's `seq_start`, `seq_end`, `count`, `key_id` and tip or digest, so
  the removal can be checked against the surviving chain.

The tombstone is written before the block disappears from the archive's
bookkeeping. A deletion that leaves no trace is indistinguishable from
tampering, and must be treated as data loss by anyone auditing the archive.

Block numbers are never reused after removal.

## 3. Verifying an archive with holes

An archive whose oldest blocks were removed by retention is still a valid
chain. Verification of such an archive:

- starts at the oldest block still present when asked for `--from 0`;
- accepts that the oldest present block has no predecessor, provided a
  tombstone explains the gap;
- states, in the report, that the first *n* blocks were removed under the
  retention policy, rather than silently starting at some block.

An unexplained hole is not the same thing as a retention deletion. If a block
inside the checked range is missing and no tombstone covers it, verification
fails, the report names the missing block number, and the command exits
non-zero.

## 4. Interaction with sealing

Removing a block never rewrites the surviving blocks: their sidecars,
signatures and `prev_tip` values keep pointing where they always did. Only
the first surviving block loses a predecessor, and its `prev_tip` continues to
record the tip of the removed block — which is what makes the tombstone
checkable.
