# Design notes

This document records the design intent of `witnesschain`: what the library is
for, which properties it is meant to provide, and how the pieces fit together.
It describes observable behaviour and invariants only. The normative byte-level
rules live in [`chain-format.md`](chain-format.md); the acceptance rules for the
verifier live in [`verification.md`](verification.md); key lifecycle lives in
[`key-handling.md`](key-handling.md).

## Purpose

A control plane makes decisions — a deployment is promoted, a tenant is
deleted, a policy is changed. After the fact, an operator or an auditor must be
able to answer three questions:

1. **What happened, in what order?** Every event is a record with a global
   sequence number, a nanosecond timestamp, the actor, the action, an opaque
   payload, and an outcome.
2. **Has anything been altered since?** The log must be tamper-evident. Any
   change to any field of any record, any deletion, any reordering must be
   detectable from the store itself.
3. **Which events are covered by a given attestation?** A checkpoint must state
   exactly which records it covers (count and sequence range) and commit to
   their contents through a Merkle root, so that a subset of records can later
   be proven to belong to it.

The library is embedded in the control-plane service. It is not a remote log;
there is no server. Integrity comes from hashing and signatures, not from access
control, and the design assumes an attacker may later have write access to the
files in the store directory.

## Threat model

In scope:

- editing a field of a stored record (including `payload` and `outcome`),
- deleting a record, a run of records, a whole segment, or the tail of the log,
- reordering or swapping segments,
- truncating the last line of a log file,
- presenting an inclusion proof for one tree against another tree's root,
- racing appends so the hash chain forks.

Out of scope: an attacker who steals the signing key *and* rewrites history
before anyone anchors a checkpoint hash externally; wholesale rollback of every
file to an older consistent snapshot (see `verification.md` §V9). These limits
are inherent to a local, key-holding store and must be mitigated operationally
by anchoring checkpoint hashes externally.

## Structure

### Records and the global chain

A record hashes all of its fields and the hash of the previous record. Hashing
every field is what makes the log tamper-evident at the finest granularity:
there is no field whose change is tolerated. The timestamp takes part at full
nanosecond precision so that two otherwise identical events in the same second
are still distinct. Sequence numbers are global and strictly consecutive; they
give the verifier a cheap, unambiguous way to name the first inconsistency.

The chain does not restart at segment boundaries. A segment is a storage and
signing unit, not a trust unit: the first record of every segment stores the
hash of the last record of the segment before it, so the chain is one continuous
sequence from the first event to the last regardless of how it is parcelled into
files.

### Segments and checkpoints

Segments bound the size of a single signing operation and of a single file read
during verification. Each sealed segment gets a checkpoint that commits to:

- the Merkle root over the segment's records,
- the record count and the first/last sequence number,
- the key that signed it,
- the hash of the previous segment's checkpoint.

The last item is what turns a sequence of checkpoints into a chain in its own
right and is what makes absence and reordering of whole segments explicit. Each
checkpoint names its predecessor by the predecessor's signed hash, so a segment
cannot be dropped or moved without breaking the link at the point where the
history no longer matches. Because the link is inside the signed message it
cannot be repaired without the key.

The Merkle root commits to the leaf set **and** its size. A root that omitted
the count would commit only to a tree shape, and a proof computed for one shape
could be presented against a root from another; binding the count makes a proof
meaningful only for the tree it was built from.

### The index and the head anchor

The index (`segments.idx`) is a convenience for operators: it lets tooling list
segments without opening and verifying every checkpoint. It is explicitly not
authoritative. If verification trusted it, an attacker could achieve whatever
they wanted by editing one small file, and an ordinary crash could make a
healthy log look broken because the index ran ahead of the data. Verification
therefore derives segment order and counts from the signed checkpoints and the
log files, and treats index disagreements as symptoms.

The head anchor duplicates the newest checkpoint. Without it, deleting the final
segment leaves a shorter but internally flawless history: every remaining
checkpoint links correctly and every record is intact. The anchor is the store's
own statement that a later segment existed. Its limitation is that an attacker
who edits the store can delete the anchor too; that is why the specification
recommends pinning the head checkpoint hash outside the store.

### Append path

Appends are concurrent in the control plane. Assigning a sequence number and the
predecessor hash must be atomic with publishing the record: two appends that
observe the same predecessor would fork the chain and duplicate a sequence
number, and no later verification could repair the ambiguity. Durability is part
of the append contract: when `Append` returns, the record and the corresponding
index entry are on disk. Buffering log bytes while advancing the index would let
a crash produce precisely the state verification is supposed to reject — an
index that describes records the log does not contain.

### Verification

Verification is a full recomputation from the beginning, not a spot check of the
newest checkpoint. It checks, in order, that each record parses and hashes
correctly, that `prev` and `seq` link consecutively, that each segment's record
count and sequence range match its signed checkpoint, that each checkpoint's
Merkle root recomputes, that every checkpoint signature verifies against the key
it names, that checkpoint hashes chain, and that the head anchor matches. The
first failure is reported with its segment and sequence number. An incomplete
final line is reported as such while the intact prefix is still verified, so an
operator learns both that the tail is bad and how far the trustworthy history
extends.

## Non-goals

- Encryption or access control on the store; the payload is opaque and stored in
  the clear.
- Compaction, rotation of the log into archives, and remote replication.
- Multi-writer coordination across processes. Concurrency is supported within
  one process holding the store open.
