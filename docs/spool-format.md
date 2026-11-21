# Spool file format and cursor semantics

This document is the normative description of how audit events are laid out on
disk at the edge and how the reader's position (the *cursor*) is maintained.
It describes required behaviour, not an implementation.

## 1. Directory layout

```
spool/
  <origin>.0.000000.ndjson
  <origin>.0.000001.ndjson
  ...
state/
  cursors/
    <origin>.0.cursor.json
```

* `spool/` holds immutable, append-only NDJSON segment files produced by the
  application. The agent never modifies or deletes them.
* `state/` holds the agent's own bookkeeping. A cursor is per
  `(origin, epoch)` pair; both terms are defined in `batch-manifest.md`.

## 2. Segment naming

A segment file name has the form:

```
<percent-encoded origin>.<epoch>.<index>.ndjson
```

* `origin` is percent-encoded with the same rules as `encodeURIComponent`, so
  it contains no literal `.` and no path separators.
* `epoch` is a non-negative integer that increases every time the producing
  application restarts its numbering.
* `index` is a non-negative integer, zero-padded to at least six digits.

Within one `(origin, epoch)`, segment `index` values are unique. Segments are
consumed in ascending `index` order. A reader must derive this order from the
numeric index, never from the order an OS directory listing happens to return.

## 3. Record encoding

* Each segment is a sequence of records separated by `\n`.
* A record is one UTF-8 JSON value with no embedded raw newline.
* The last record in a segment **may** end with a trailing `\n` or may end at
  EOF without one. Both forms denote the same complete record. A segment whose
  bytes do not end in `\n` still has a readable final record.
* A record longer than the configured `maxRecordBytes` is invalid. The reader
  must reject it as an error (naming the file and byte offset) rather than
  emitting a shortened record. Truncation is never permitted: the collector
  compares the manifest `count` and `digest` against exactly the bytes that
  were sent, and a shortened record cannot be matched back to the spool.
* An empty trailing region after the final `\n` is not a record.

## 4. Sequence numbers

Records are assigned a sequence number when they are appended, monotonically
increasing within `(origin, epoch)`, starting at `0` and with no holes. The
sequence number of a record is a property of the stream, not of the segment:
it continues across segment boundaries without resets.

## 5. Cursor file

A cursor file is a JSON document:

```json
{
  "version": 1,
  "origin": "web-01#a",
  "epoch": 3,
  "segment": "web-01%23a.3.000002.ndjson",
  "offset": 418,
  "nextSeq": 57,
  "ackedSeq": 56,
  "updatedAt": "2026-11-20T09:14:31.005Z"
}
```

* `segment` is the file the reader will resume in.
* `offset` is a byte offset into that file. It is always either `0` or the
  position immediately after a consumed `\n`, so a resumed reader can decode a
  whole record from it.
* `nextSeq` is the sequence number the next delivered record will carry.
* `ackedSeq` is the highest sequence number the collector has acknowledged.

### 5.1 Meaning of the cursor

The cursor names the **first record that has not yet been acknowledged by the
collector**. Every record before it has been durably accepted by the
collector; every record at or after it has not. The cursor is a delivery
position, not a read position.

Consequently:

* Opening a cursor that does not exist starts at the beginning of the stream:
  `segment` is the lowest-indexed segment for the stream and `offset` is `0`.
  Existing records in the spool are not skipped.
* A cursor may only move forward after a 2xx response for the batch that covers
  the records being skipped. It must never move forward on the strength of
  having read records from the spool.
* A cursor may be moved backwards only by an explicit administrative reset;
  normal delivery never rewinds it.

### 5.2 Durability

Cursor updates are ordered as follows:

1. Read the batch from the spool.
2. Send the batch and receive a 2xx response.
3. Write the new cursor.
4. `fsync` the cursor file, and `fsync` the containing directory if the file
   was newly created, before any subsequent batch is read.

A crash at any point before step 4 completes must leave the previous cursor
intact or the new one on disk. It must never leave a cursor that claims
records were delivered when no 2xx was received. If the cursor is missing or
older than the last successful delivery, the agent re-sends from that point;
the collector's deduplication makes the resend harmless.

The ordering and the `fsync` are both part of the contract: the cursor is the
agent's only durable record of what the collector has confirmed, and a crash
must leave it at or behind the collector's accepted watermark.

## 6. Batch reading

* One batch covers contiguous records from one `(origin, epoch)` and contains at
  most `limit` records.
* A batch never spans two origins or two epochs.
* The manifest's `count` is the number of complete records in the batch and its
  `digest` is computed over the exact payload bytes sent. Both are derived
  after the reader has decided which records are complete; an incomplete or
  over-long record must not be counted or digested in a shortened form.
* A final record with no trailing `\n` is a complete record and is included in
  the batch, with or without the trailing separator in the emitted payload, as
  long as the transmitted bytes match the digest. It must never be silently
  dropped.
* Reading stops at the first record that cannot be read in full. A record that
  is still being appended may be left for a later batch; the reader's decision
  must be consistent from batch to batch, and it must never emit a truncated
  line.
