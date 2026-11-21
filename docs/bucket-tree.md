# Bucket and directory trees

This document states the intended behaviour of the two Merkle layers used
by subsetwitness. It is normative: an implementation that disagrees with a
clause below produces commitments that other parties cannot reproduce.

## §1 Scope

A disclosure period covers one fixed-length time slice (one hour by
default). Within a period every event is assigned to exactly one actor
bucket. A bucket is a Merkle tree over that actor's events. A directory
tree is a second Merkle tree over all buckets of the period.

## §2 Leaf encoding

### §2.1 Canonical serialization

The bytes hashed into a bucket leaf must depend only on the semantic
content of the record, never on how the record was typed or stored.

* Keys must be emitted in a fixed order that does not depend on insertion
  order, dictionary ordering, or the writer's language runtime.
* Separators, whitespace and escaping must be fixed. Non-ASCII text must
  be encoded deterministically and must not change when the same value is
  written with or without escaping.
* Two records that are equal as values must serialize to identical bytes.

The leaf digest is the hash of a fixed domain tag followed by those bytes.

### §2.2 Fields covered

A bucket leaf covers every field that identifies the event and its place
in the ledger. At minimum:

* the actor,
* the sequence number,
* the timestamp,
* the action and any other payload fields present,
* the per-actor record count associated with the bucket.

The actor field must be inside the hashed bytes. Moving an event between
two actors' buckets must change both buckets' roots.

Because the count is covered by the leaf, a bucket commitment binds the
number of records as well as their contents. A party cannot claim a
different count for the same root.

## §3 Leaf ordering

### §3.1 Sequence order

Leaves within a bucket are ordered by sequence number in numeric order.
`seq` is an integer in the schema; ordering must not be done on its string
form. Consequently `seq` 2 precedes `seq` 10, and the order is independent
of the order in which records were appended or read.

No two records in one actor's bucket may share a sequence number. A
duplicate is a malformed bucket and must be rejected rather than silently
ordered.

## §4 Bucket root

### §4.1 Empty bucket

An empty bucket has no leaves. It must be represented by a distinct,
well-defined sentinel root that cannot be produced by any non-empty set of
records. The sentinel must use its own domain separation, so that it is
distinct from every possible leaf digest and from every possible internal
node digest.

Empty buckets are legitimate at the directory layer: a period can contain
no events for a given actor only if that actor is omitted from the
directory entirely. A bucket that is present in a directory proof always
has at least one leaf.

### §4.2 Single-record bucket

A bucket with one record has a root equal to that record's leaf digest.
Its inclusion proof is empty, and verification must still compare the leaf
digest against the claimed root.

### §4.3 Level combination

At each level, adjacent nodes are combined pairwise in left-to-right order.
When a level has an odd number of nodes, the last node is carried up
unchanged. The rule must be applied identically when building and when
verifying; the verifier must not re-derive a different pairing.

A record's inclusion proof is the ordered list of sibling digests from the
leaf level upward. Each proof entry must state on which side the sibling
sits, and verification must place the accumulated value on the opposite
side.

## §5 Directory tree

### §5.1 Directory leaf content

A directory leaf commits to one bucket. It must cover, at minimum:

* the bucket's actor name,
* the bucket root,
* the bucket record count.

The actor name must be inside the hashed bytes. Without it, the bucket of
one actor is byte-for-byte interchangeable with the bucket of another, and
a disclosure for one account can be presented as a disclosure for a
different account.

### §5.2 Bucket ordering

Buckets within one period are ordered by actor name using a fixed
collation. The ordering must be total and must not depend on hash-map
iteration order, insertion order, or locale.

### §5.3 Padding slots

When the number of real buckets is not a power of two, the level is
completed to the next power of two with padding slots.

* A padding slot must be distinguishable from a real bucket. Its digest
  must use a distinct domain tag or an explicit marker, and it must never
  equal the digest of a real bucket.
* A padding slot is not a bucket. A directory proof for a padding slot is
  invalid and must be rejected: the verifier must be able to tell that the
  claimed position holds no bucket.
* Padding must be deterministic, so that the period root is the same for
  every writer that pads the same set of buckets.

## §6 Directory proof

### §6.1 Sibling orientation

A directory proof is the ordered list of sibling digests from the bucket's
leaf level to the period root. Every entry must carry its side. The
verifier must combine according to the recorded side: if the sibling is on
the left, the accumulated value goes on the right, and vice versa.

Reordering the entries, or swapping left and right, must change the
recomputed root and must therefore fail. A verifier that ignores the side
labels accepts a tree whose internal node ordering has been rearranged.

### §6.2 Proof length and position

The proof length is determined by the tree height, which is determined by
the padded bucket count.

* The verifier must know the number of buckets in the period, either from
  the package or from the commitment, and must check the proof length
  against it.
* The verifier must check the claimed index against the number of buckets
  and use it to drive the fold.
* An empty proof is valid only for a single-bucket period, and then only
  when the leaf digest equals the period root.
* Truncating a proof must not verify; extending it must not verify.

The proof must also be checked against the binding in the period
commitment: the actor, root and count presented in the proof must equal
the values committed to in the chain (§4 of `disclosure.md`).
