# Journal format

This document specifies the on-disk representation of an audit record, how a
record is serialised canonically, how a commitment is computed, and how
records are chained.

## 1. Record model

A record has the following logical members:

| Member | Type | Meaning |
| --- | --- | --- |
| `seq` | integer | 1-based position in the journal |
| `actor` | string | identity that performed the action |
| `action` | string | action name |
| `ts` | string | RFC 3339 timestamp in UTC, nanosecond precision |
| `prev_hash` | string | hash of the preceding record |
| `fields` | object | structured payload |
| `hash` | string | this record's hash |

`fields` is a JSON-like value tree: objects with string keys, arrays, strings,
integers, floating point numbers, booleans and `null`. A leaf that the policy
redacts is replaced by a commitment object:

```json
{
  "commitment": "<64 lowercase hex characters>",
  "redacted": true,
  "salt_id": "<16 to 64 lowercase hex characters>"
}
```

The commitment object carries exactly these three members. `redacted` is
always `true` for a redacted leaf.

## 2. Canonical encoding

A record is encoded as a single-line JSON object with **members in a fixed
order**:

```
seq, actor, action, ts, prev_hash, fields, hash
```

Within `fields`, object keys are emitted in ascending byte order of their
UTF-8 representation. Array elements keep their index order. The commitment
object's members are emitted in the order `commitment`, `redacted`, `salt_id`.

The encoding is deterministic: encoding the same logical record twice must
produce byte-identical output. No whitespace is emitted between tokens.

### 2.1 String encoding

Strings are encoded as UTF-8 **verbatim**. Multibyte characters — Chinese,
emoji, accented Latin letters, and any other valid UTF-8 sequence — appear
as their UTF-8 bytes in the output. They MUST NOT be converted to `\uXXXX`
escapes, and surrogate pairs MUST NOT appear in the output for characters
that have a direct UTF-8 representation.

Only the characters that JSON requires to be escaped are escaped:

- `"` → `\"`
- `\` → `\\`
- U+0008 → `\b`
- U+000C → `\f`
- U+000A → `\n`
- U+000D → `\r`
- U+0009 → `\t`
- other code points below U+0020 → `\u00XX`

Characters above U+001F are written as-is, including `/` and non-ASCII
letters.

### 2.2 Numbers

Integers are written in base 10 without a leading `+` and without leading
zeros. Floating point values use the shortest representation that round-trips
to the same `float64`, with a `.0`-free integral form written as an integer
token.

## 3. Commitments

For a leaf value `v` at canonical field path `p`, redacted under salt `s`:

```
commitment = HMAC-SHA256(key = s, message = p || 0x00 || canonical(v))
```

`canonical(v)` is the canonical encoding of the leaf value alone, following
§2: strings verbatim UTF-8, objects with sorted keys, arrays in order.

- The salt `s` is 32 bytes drawn from a cryptographically secure random
  source. A fresh salt is generated for every redacted leaf.
- `salt_id` is an identifier derived independently of the salt value and is
  used to locate the salt in the keystore. It is not secret.
- The commitment is the lowercase hex encoding of the HMAC output (64
  characters).

The salt itself is **never** written into the journal. It is stored in the
keystore entry named by `salt_id`, together with a sealed (encrypted) copy of
the original leaf so that the value can be recovered after approval.

## 4. Hashing and chaining

The hash of record `r` is:

```
hash = SHA-256(
    "maskseal-journal/v1" || 0x00 ||
    "seq="    || decimal(r.seq)    || "\n" ||
    "actor="  || r.actor           || "\n" ||
    "action=" || r.action          || "\n" ||
    "ts="     || r.ts              || "\n" ||
    "prev="   || r.prev_hash       || "\n" ||
    "field="  || p1 || "=" || canonical(v1) || "\n" ||
    "field="  || p2 || "=" || canonical(v2) || "\n" ||
    ...
)
```

Field lines are emitted in ascending byte order of the canonical field path.
The canonical value for **every** leaf participates in the hash input:

- a plain leaf contributes its canonical encoding;
- a redacted leaf contributes its **commitment object** exactly as it appears
  in the record, i.e. its `commitment`, `redacted` and `salt_id` members.

A redacted leaf MUST NOT be treated as absent from the hash input, and the
`redacted` flag and `salt_id` MUST be covered by the hash. Consequently,
altering a commitment, flipping `redacted` from `true` to `false`, or
swapping a `salt_id` on an already-written record changes the computed hash
and makes verification fail.

The hash field itself is not part of the hash input. Everything else in the
record is.

## 5. Chain

The first record in a journal uses

```
prev_hash = "0000000000000000000000000000000000000000000000000000000000000000"
```

Each subsequent record uses the `hash` of the record before it as its
`prev_hash`.

Verification walks records in `seq` order and must check, for every record:

1. the stored `hash` equals the recomputed hash (§4);
2. the stored `prev_hash` equals the hash of the preceding record (or the
   genesis value for seq 1);
3. `seq` increases by exactly one from one record to the next.

If all three hold for every record, the journal verifies.

## 6. Verification outcome

Verification reports the **first** sequence number at which any check fails.
That sequence number is reported to the caller, and the verifier returns a
non-zero error. A journal whose chain is intact returns no error.

The command `maskseal verify` exits with status 0 when verification returns no
error and with a non-zero status otherwise.

## 7. Journal directory

```
<journal>/
  journal.log   one canonical encoded record per line, "\n" terminated
  journal.idx   one line per record: "<seq> <offset> <length>"
```

`offset` is the byte offset of the first byte of the record line in
`journal.log`; `length` includes the terminating newline. The index is a
derived structure: it may be rebuilt from the data file at any time. The data
file is the source of truth.

## 8. Durability ordering

The data file and the index must be kept consistent with respect to process
termination:

- the index entry for a record MUST NOT be visible before that record's bytes
  are present in the data file;
- a partially written record MUST NOT be counted as part of the journal;
- after the process is terminated at any point, the index must never describe
  more records than the data file contains, and verification must not report a
  successfully written record as missing.

Batched or buffered writes are an implementation detail, but the observable
ordering above is required.
