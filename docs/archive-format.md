# Archive format

This document describes the on-disk shape of a rotseal archive and the rules a
correct implementation must satisfy. It is normative: the numbers, field names
and ordering rules below are what `verify` is expected to enforce.

## 1. Archive layout and naming

```
<archive>/
  active.ndjson                # the log the application is still writing to
  index.json                   # rotation bookkeeping
  segments/
    segment-000001.ndjson      # a closed, immutable block
    segment-000001.seal        # its sidecar
    segment-000002.ndjson
    segment-000002.seal
```

- Segment files are named `segment-<NNNNNN>.ndjson` with a zero-padded,
  six digit, positive block number.
- A segment's sidecar is named `segment-<NNNNNN>.seal` and always sits next to
  the segment it describes.
- Numbers are allocated monotonically. A block number is never reused, not
  even after the block has been deleted by retention.
- Blocks are numbered from 1. Block 0 does not exist; `--from 0` means "start
  at the oldest block still present".

## 2. Segment contents

### 2.1 Whole records only

A segment is a sequence of complete NDJSON records, one per line, each line
terminated by `\n`. A rotation boundary always falls *between* two records.

The file is UTF-8. No line may begin or end in the middle of a multibyte
character; reading any segment with a strict UTF-8 decoder must succeed. A
rotation strategy expressed in bytes therefore has to move a boundary to a
record boundary rather than cutting at an arbitrary offset.

### 2.2 Record fields

Every record is a JSON object carrying at least:

| field | meaning |
|---|---|
| `seq` | monotonically increasing integer sequence number |
| `ts` | RFC 3339 timestamp with an explicit offset |
| `level` | log level |
| `msg` | message text |

### 2.3 Sidecar counters describe the block

The sidecar records `count` (number of records) and `sealed_length` (number of
bytes in the segment file at sealing time). For a block to verify, both values
must equal the corresponding quantity measured from the file on disk at
verification time. Extra bytes at the end of a sealed block are a violation
even if every original record is still intact.

### 2.4 Chaining

Each sidecar carries `prev_tip`, the chain tip of the block immediately
preceding it. The tip of a block is derived from that block's identity and
content, and is recomputable by anyone holding the block and its sidecar. The
first block of an archive has `prev_tip` set to `null`.

## 3. Sequence continuity

### 3.1 Within a block

`seq` values inside one segment increase strictly, one record at a time.
`seq_start` and `seq_end` in the sidecar are the first and last value found in
the file, and every value between them must be present exactly once.

### 3.2 Across blocks

For consecutive blocks *n* and *n+1*, `seq_end` of block *n* plus one equals
`seq_start` of block *n+1*. Sequence ranges must never overlap.

Timestamps are compared as instants, not as local clock readings. When the
local clock repeats an hour (a daylight-saving transition), records from the
two occurrences of that hour must still receive distinct, monotonic `seq`
values and must not be cut twice.

### 3.3 A missing block is a gap

If a block that the archive says existed is absent from disk, that is a gap,
not a skipped block. Verification reports the number of every absent block.

## 4. Index

`index.json` is the archive's own registry of blocks:

```json
{
  "version": 1,
  "segments": [
    {"number": 1, "file": "segment-000001.ndjson", "mode": "size",
     "bytes": 4096, "rotated_at": "2026-01-01T00:00:00Z",
     "sealed": true, "key_id": "key-1a2b3c4d",
     "count": 40, "seq_start": 1, "seq_end": 40, "sealed_length": 4096}
  ]
}
```

### 4.1 Index versus disk

The index is bookkeeping, not authority. Verification cross-checks it against
the files on disk: a numbered block that is listed but absent, or present but
unlisted, must be reported rather than silently ignored.

### 4.2 Ordering

Blocks are ordered by their number, not by their position in the index array.
Gaps in the numbering are meaningful and must be surfaced, never stepped over.
