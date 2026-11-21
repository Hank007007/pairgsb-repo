# Chain format specification

This document is normative. It defines the bytes that `witnesschain` writes to
disk and the exact computations that give those bytes meaning. Terms written in
**MUST**, **MUST NOT**, **SHOULD**, and **MAY** carry their usual RFC 2119
weight. Clause numbers are stable and are referenced from the verification
specification.

## §1 Scope and vocabulary

- **Record**: one audit event, serialized as a single JSON object on one line.
- **Segment**: an ordered run of records stored in one log file, terminated by
  a checkpoint.
- **Leaf hash**: the Merkle-tree leaf derived from a record hash.
- **Checkpoint**: a signed statement about one segment (its record count, its
  Merkle root, and the identity of the preceding checkpoint).
- **Store**: a directory containing keys, segments, an index, and a head
  checkpoint.

All hexadecimal strings in this specification are lowercase, without a `0x`
prefix. All integers in hash inputs are unsigned decimal without leading zeros.
All hashes are SHA-256, rendered as 64 hexadecimal characters. All signatures
are Ed25519.

## §2 Record serialization

§2.1 A record is serialized as compact JSON with exactly these members, in this
order:

```
{"seq":<uint64>,"ts":"<RFC 3339 timestamp>","actor":"<string>",
 "action":"<string>","payload":"<string>","outcome":"<string>",
 "prev":"<64 hex>","hash":"<64 hex>"}
```

Whitespace between members is not significant for parsing, but writers MUST
emit the order above so that byte-for-byte comparison of a record line is a
valid integrity check.

§2.2 `seq` is a positive integer. The first record ever written to a store has
`seq` 1, and every later record has `seq` exactly one greater than the record
before it in the global log order. Sequence numbers are global to the store,
not per segment.

§2.3 `ts` is a UTC timestamp encoded with RFC 3339 **nanosecond** precision
(the `time.RFC3339Nano` layout). Writers MUST retain the full nanosecond value;
readers MUST NOT reduce it to a coarser unit before performing any hash or
comparison.

§2.4 `actor`, `action`, `payload`, and `outcome` are opaque UTF-8 strings.
`payload` is a string (typically JSON text) and is not interpreted by the
library. Empty values are permitted and MUST be hashed as empty strings, not
omitted.

§2.5 `prev` is the `hash` of the immediately preceding record in the global
log order. The first record of a store has `prev` equal to sixty-four `0`
characters (the **zero hash**). A record that begins a new segment after any
previous record MUST use the previous record's hash as `prev`; segment
boundaries do not reset the chain.

§2.6 `hash` is the record hash defined in §3.

## §3 Record hash

§3.1 The record hash is the SHA-256 digest of the following byte sequence. Each
element is written literally, fields are separated by a single LF (`\n`), and
the sequence ends with a final LF:

```
witnesschain/v1/record\n
<seq>\n
<ts in RFC 3339 nanosecond UTC form>\n
<actor>\n
<action>\n
<payload>\n
<outcome>\n
<prev>\n
```

The literal first line is a domain-separation tag and is part of the input.

§3.2 Every field in §2.1 except `hash` itself participates in the hash input.
In particular `payload` and `outcome` are covered, so changing either value
changes the record hash and every structure derived from it.

§3.3 Because the input uses LF separators and an explicit final LF, different
field combinations cannot produce the same input by concatenation. Writers
MUST NOT substitute a different separator or omit the trailing LF.

§3.4 The timestamp contributes its full nanosecond precision, so two records
that differ only below the second are different records with different hashes.
Writers MUST format the timestamp in UTC to avoid offset-dependent hashes for
the same instant.

§3.5 Readers recomputing a record hash MUST use the value of `seq`, `ts`,
`actor`, `action`, `payload`, `outcome`, and `prev` exactly as stored, and MUST
compare the result to the stored `hash` with a constant-time comparison.

## §4 Global hash chain

§4.1 Records form a single chain across the whole store. Record *i* stores the
hash of record *i-1* in `prev`; the first record stores the zero hash.

§4.2 A segment is a contiguous slice of this chain. The first record of segment
*N* (*N* > 1) MUST continue the chain: its `prev` equals the hash of the last
record of segment *N-1*. A checkpoint boundary MUST NOT introduce a fresh
all-zero `prev`.

§4.3 Two records with the same `prev` would create a fork. The writer MUST
ensure that each record's `prev` is the hash of the record most recently made
durable, and that sequence assignment and record publication are atomic with
respect to concurrent appenders.

## §5 Segments and file naming

§5.1 Segment numbers are positive integers, starting at 1, with no gaps in a
healthy store.

§5.2 Records of segment *N* are stored in `segments/<NNNNNN>.log`, where
`<NNNNNN>` is the segment number zero-padded to six decimal digits. Each record
occupies one line terminated by LF.

§5.3 The checkpoint of segment *N* is stored in `segments/<NNNNNN>.ckpt` as one
JSON object (see §7). The log file and checkpoint file for a segment MUST both
exist for the segment to be considered present.

§5.4 A new segment starts when the current segment reaches its configured
record limit. A store MAY also seal early; the limit is not part of the
integrity rules.

## §6 Merkle tree

The Merkle tree is built over the segment's records in log order. Leaf *i* is
derived from the `hash` of the *i*-th record.

§6.1 **Leaf hash.** The leaf hash is

```
SHA-256(0x00 || lowercase_hex(record_hash))
```

where `lowercase_hex(record_hash)` is the 64 ASCII characters of the record
hash. The `0x00` byte is a domain-separation prefix written as one raw byte.

§6.2 **Parent hash.** The parent of a left child `L` and a right child `R` is

```
SHA-256(0x01 || lowercase_hex(L) || lowercase_hex(R))
```

The order of children is significant: swapping `L` and `R` produces a different
parent. The `0x01` byte is a domain-separation prefix written as one raw byte.

§6.3 **Tree shape.** Nodes at each level are paired left-to-right. If a level
has an odd number of nodes, the final node is carried to the next level
unchanged (it is not duplicated and not hashed with itself). Levels are
combined until a single top node remains.

§6.4 **Root binding.** The segment root is

```
SHA-256("witnesschain/v1/root" || 0x00 || uint64_be(count) || lowercase_hex(top))
```

where `count` is the number of leaves, encoded as eight bytes big-endian, and
`top` is the node remaining after §6.3. A tree with `count` 0 has no root and a
segment MUST contain at least one record. The root therefore commits to both
the leaf set and its size: the same top node under a different leaf count yields
a different root.

§6.5 **Inclusion proofs.** A proof for leaf index *i* in a tree of *count*
leaves is an ordered list of steps, one per tree level. At each level:

- If the current node has a sibling, the step contains the sibling hash and a
  side flag: `left` is true when the sibling is the left child.
- If the current node is the unpaired final node of an odd level (carried up
  under §6.3), the step contains no sibling and is marked as a promotion.

The number of steps equals the number of levels, which is `ceil(log2(count))`
for `count > 1` and 0 for `count == 1`. The side of every step is fully
determined by the leaf index and the level; a proof that carries a side flag
inconsistent with the index is invalid.

## §7 Checkpoint

§7.1 A checkpoint is a JSON object with exactly these members, in this order:

```
{"version":1,"segment":<uint64>,"count":<uint64>,"first_seq":<uint64>,
 "last_seq":<uint64>,"root":"<64 hex>","prev_checkpoint":"<64 hex>",
 "key_id":"<string>","signature":"<base64>"}
```

§7.2 `version` is 1. `segment` is the segment number. `count` is the number of
records in the segment. `first_seq` and `last_seq` are the sequence numbers of
the segment's first and last records. `root` is the Merkle root from §6.4.

§7.3 `prev_checkpoint` is the **checkpoint hash** of the previous segment's
checkpoint. For segment 1 it is the zero hash. The checkpoint hash is defined in
§7.5. This field binds each checkpoint to its predecessor and thus to the entire
history before it.

§7.4 The signed message is the UTF-8 byte sequence

```
witnesschain/v1/checkpoint\n
segment=<segment>\n
count=<count>\n
first_seq=<first_seq>\n
last_seq=<last_seq>\n
root=<root>\n
prev_checkpoint=<prev_checkpoint>\n
key_id=<key_id>\n
```

`signature` is the base64 (standard encoding, with padding) of the Ed25519
signature over exactly those bytes. A verifier MUST reject a checkpoint if any
signed field differs from the value that was signed, if the signature does not
verify under the public key named by `key_id`, or if the key is unknown.

§7.5 The checkpoint hash is `SHA-256(signed_message)`, rendered as lowercase
hex. It covers everything in §7.4 and excludes `signature` itself (which is
deterministic for Ed25519 in any case).

## §8 Index file

§8.1 `segments.idx` is a JSON Lines file. Each line is an object with members
`segment`, `file`, `checkpoint`, `count`, and `last_seq`. It is a convenience
index for operators and tooling.

§8.2 The index is **not** authoritative. The authoritative description of a
segment is its signed checkpoint plus the log file. A verifier MUST derive
segment order, segment counts, and chain linkage from checkpoints and log files,
and MUST NOT accept an index claim that conflicts with a signed checkpoint. An
index that lags behind or omits lines MUST NOT cause an otherwise valid store to
be reported as damaged, and an index that claims records that are not durably
present MUST NOT cause those records to be treated as present.

§8.3 Writers update the index only after the corresponding log bytes have been
flushed durably. The index therefore never describes more records than the log
contains at the moment it is written.

## §9 Head anchor

§9.1 `segments/HEAD.ckpt` is a copy of the most recently sealed checkpoint,
byte-identical to that segment's `.ckpt` file.

§9.2 The head anchor exists so that removal of the final segment is detectable:
the remaining segments form a valid chain, but the store still claims, through
the signed head checkpoint, that segment *N* exists. A store with at least one
sealed segment MUST have a head anchor.

§9.3 Removing the head anchor itself removes this guarantee; deployments that
need to detect tail truncation independent of the local filesystem MUST pin the
latest checkpoint hash outside the store (see `docs/verification.md` §V9).

## §10 Durability

§10.1 An append is acknowledged only after its record line and the updated
index entry have been written and flushed to the operating system. A process
that is killed before that point may leave a partial final line; it MUST NOT
leave an index entry describing the unacknowledged record.

§10.2 `segments.idx` describes only durable records. Buffering of log bytes is
permitted only if the buffer is flushed before the index is advanced and before
an append is acknowledged.

§10.3 On startup for appending, a writer MUST discard an incomplete final line
(one without a terminating LF) rather than appending after it, so that the log
remains a sequence of complete JSON lines.
