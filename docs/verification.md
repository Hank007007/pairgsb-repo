# Verification specification

This document defines what `witnessctl verify <dir>` must establish and how it
must report failures. It is normative. Clause numbers are stable. Format
clauses are cited as `§V…` here and `§…` in `docs/chain-format.md`.

## §V1 Outcome model

§V1.1 Verification either succeeds, or fails at the **first** inconsistency in
global log order. The command reports that position and exits non-zero.

§V1.2 A failure report contains, when known:

- the segment number,
- the sequence number,
- a short category, and
- a human-readable explanation naming the expected and observed values.

§V1.3 Verification MUST NOT stop silently at the first malformed line and
report success for the remainder. Every structural problem it encounters is
either reported or is provably harmless under this specification.

## §V2 Inputs

§V2.1 The verifier reads the segment list, the segment log files, the segment
checkpoints, the head anchor, and the public keys under `keys/`.

§V2.2 The verifier MUST NOT use `segments.idx` as the source of truth for
segment membership, counts, or sequence ranges. It MAY use it for diagnostics,
and it MUST report an index/checkpoint conflict, but the conflict itself is a
symptom; the authority is the checkpoint (see `§8.2`).

§V2.3 Segment files present under `segments/` are enumerated directly. Both
`<NNNNNN>.log` and `<NNNNNN>.ckpt` MUST exist for a segment to be verified.
A `.log` without a `.ckpt`, or a `.ckpt` without a `.log`, is a failure.

§V2.4 A segment whose `.ckpt` is present but whose `.log` is absent is reported
as a missing-segment failure at the segment's expected first sequence number,
not skipped.

## §V3 Record-level checks

For each record line, in order, the verifier MUST:

§V3.1 Parse the line as one JSON object. A line that does not parse as a record
is a failure at that line's position, with category `malformed-record`. Blank
lines between records are a failure. (A trailing incomplete line is handled by
§V6.)

§V3.2 Recompute the record hash per `§3` from every stored field and compare it
to the stored `hash`. A mismatch is `hash-mismatch`. This includes a change to
`payload`, `outcome`, `ts`, `actor`, `action`, `seq`, or `prev`.

§V3.3 Check `prev` equals the previous record's hash (the zero hash for the
first record of the store). A mismatch is `prev-mismatch`, reported with the
last hash as expected and the stored `prev` as observed.

§V3.4 Check `seq` equals the previous record's `seq` plus one, with the first
record's `seq` equal to 1. A gap, duplication, or decrease is `seq-out-of-order`.

§V3.5 Check that record fields match the shape in `§2.1`: the right member set,
string types where strings are required, and a 64-character lowercase hex
`prev` and `hash`. Wrong types or a missing member are `malformed-record`.

## §V4 Segment-level checks

§V4.1 Parse the segment checkpoint. If it is missing or unparseable, fail at the
segment with `malformed-checkpoint`.

§V4.2 Verify the checkpoint signature per `§7.4` using the public key named by
`key_id`. If the key is unknown, fail `unknown-key`. If the signature does not
verify, fail `bad-signature`. Verification of the signature is mandatory for
every segment, not only the last.

§V4.3 Check the segment number in the checkpoint matches the file's segment
number. A mismatch is `checkpoint-mismatch`.

§V4.4 Check the number of parsed records equals the checkpoint `count`. This is
the primary defence against tail truncation and against index lies. A mismatch
is `count-mismatch`, reporting the signed count as expected and the number of
complete records found as observed.

§V4.5 Recompute the Merkle root per `§6.4` over the parsed records. Compare to
the checkpoint `root`. A mismatch is `root-mismatch`.

§V4.6 Check `first_seq` equals the segment's first record's `seq` and
`last_seq` equals its last record's `seq`, and that `first_seq` continues the
global sequence from the previous segment's `last_seq`. A mismatch is
`seq-range-mismatch`.

§V4.7 Check the checkpoint's `prev_checkpoint` equals the checkpoint hash
(`§7.5`) of the previous segment's checkpoint, or the zero hash for segment 1.
This binds segments in order and makes deletion or reordering of whole
segments detectable. A mismatch is `checkpoint-chain-mismatch`.

## §V5 Cross-segment checks

§V5.1 The first record of segment *N* for *N* > 1 MUST have `prev` equal to the
last record hash of segment *N-1* (`§4.2`). A first record pointing at the zero
hash mid-chain is `prev-mismatch`.

§V5.2 Segment numbers encountered MUST be contiguous starting at 1. A gap is
`missing-segment`; the report identifies the first missing number and the
sequence range that cannot be accounted for.

§V5.3 Segments found out of numeric order, or whose checkpoints' `prev_checkpoint`
links form a cycle or a skip, are `checkpoint-chain-mismatch` (see `§V4.7`).

## §V6 Incomplete final line

§V6.1 If the final segment's log file does not end with LF, the final partial
line is **incomplete**. Verification reports `truncated-tail`, naming the
segment and the sequence number the complete records would reach, and treats the
file as containing only the complete lines before it.

§V6.2 Because an incomplete tail is a real durability fault, `truncated-tail`
alone is a **failure** (non-zero exit). The requirement that the complete
records are retained means: the verifier still validates every complete record
and every other segment, and reports the tail problem in addition to, not
instead of, any earlier failure it finds. It MUST NOT discard or refuse to check
the intact prefix.

§V6.3 A partial line may also occur in the middle of a file if the file was
concatenated or corrupted; that is `malformed-record` at §V3.1, because only the
final unterminated line qualifies as an incomplete tail.

## §V7 Root binding and proof checks

§V7.1 When an inclusion proof is supplied for a leaf, its verification MUST use
the index-determined sides from `§6.5`, the exact step count for the tree depth,
and the count-bound root from `§6.4`.

§V7.2 A proof from a smaller tree presented against a root computed over a
larger tree MUST fail, because the root commits to the leaf count `§6.4`.

§V7.3 A proof whose steps are reordered or whose sides are swapped MUST fail,
because parent inputs are ordered `§6.2`.

§V7.4 A proof with fewer or more steps than the depth of the tree MUST fail.

## §V8 Head anchor check

§V8.1 If `segments/HEAD.ckpt` is present, it MUST be byte-identical to the
checkpoint of the highest-numbered segment present. A mismatch is
`head-stale`; the report identifies both checkpoints.

§V8.2 If the highest-numbered present segment is lower than the segment named by
the head anchor, at least one trailing segment has been removed; this is
`missing-segment` reported at the head segment's number.

§V8.3 If no sealed segment exists, the head anchor MAY be absent.

## §V9 What verification cannot establish

§V9.1 Verification over files in a single directory cannot, by itself, prove
that a store has not been rolled back wholesale (all files replaced by an older
consistent snapshot). Detecting rollback requires pinning the latest checkpoint
hash in an external, append-only system and comparing `§V8`.

§V9.2 Verification cannot recover a log whose segments and checkpoints were all
removed together with their keys. Such a store verifies as empty.

## §V10 Reporting format

§V10.1 On success the command prints a single summary line including the number
of segments, the number of records, and the head checkpoint hash.

§V10.2 On failure the command prints a line beginning `FAIL` followed by the
category and location, then a line beginning `  expected:` and one beginning
`  observed:`. If more than one independent problem is found, only the first is
required; additional findings MAY be summarized after it.

§V10.3 The exit status is 0 on success and non-zero on any failure.
