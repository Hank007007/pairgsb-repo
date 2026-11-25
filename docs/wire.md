# protolens wire format

This document describes the wire-level rules protolens must honour when it
reads and writes protobuf-encoded payloads. It states required behaviour only;
it does not prescribe code structure.

## 1. Scope

protolens implements a strict subset of the protobuf binary wire format that is
sufficient to carry gateway requests: varints, length-delimited byte strings,
and the scalar and message field types listed in §3. Anything outside that
subset is a protocol error and must be rejected rather than guessed at.

## 2. Varints, tags and framing

### 2.1 Varints

A varint is a sequence of bytes where every byte except the last has its most
significant bit set. The payload is the concatenation of the low 7 bits of each
byte, least-significant group first.

The following are protocol errors:

- **Too long.** A varint occupies at most **10** bytes. A reader that has
  consumed ten bytes and still sees the continuation bit set must fail, and
  must not consume an eleventh byte.
- **Non-minimal encoding.** A varint whose final byte is `0x00` while it is not
  the first byte (for example `0x80 0x00`) is not a legal encoding of zero and
  must be rejected. More generally, any encoding that could be shortened
  without changing the value is invalid.
- **Truncated.** A varint that ends at the end of the input while the
  continuation bit is still set is a protocol error.
- **Out of range.** The decoded value must fit in an unsigned 64-bit integer
  (0 .. 2^64-1). Ten bytes whose payload bits form a value above 2^64-1, and in
  particular a tenth byte carrying more than the single bit that fits, is a
  protocol error.
- **Writers.** A writer must emit the shortest possible encoding for every
  value it writes, including negative signed values, and must never emit more
  than ten bytes. A writer given a negative value in a signed 32-bit field
  encodes it as a 64-bit two's-complement varint (ten bytes), exactly as the
  protobuf specification requires.

Every error in this section is reported as a decoding error; none of them is
allowed to produce a partial or silently truncated value.

### 2.2 Tags

Each field on the wire begins with a tag varint:

```
tag = (field_number << 3) | wire_type
```

Both halves are validated before any field payload is read.

### 2.3 Field numbers and wire types

- A field number must be between **1** and **2^29-1** inclusive. Field number
  `0` is not legal and must be rejected.
- The wire types are: `0` varint, `1` fixed64, `2` length-delimited, `5`
  fixed32. Wire types `3` and `4` (the deprecated start-group / end-group
  markers) are **rejected** outright; protolens does not implement groups.
  Wire types `6` and `7` do not exist and are rejected.
- A tag whose wire type is not one of the four legal values must be reported as
  an error, never skipped as though it were an ordinary unknown field.

### 2.4 Length-delimited fields

A length-delimited field is a varint byte count followed by exactly that many
bytes.

- The declared length is compared against the number of bytes actually
  remaining before the payload is sliced. If the declared length is greater
  than the remaining bytes the input is truncated and the field must be
  rejected. A reader must never silently clamp the length to what is available.
- A declared length that cannot itself be represented in the remaining input
  (including lengths that exceed the safe integer range) is an error.

### 2.5 Nesting

- A message may be nested at most **16** levels deep, counting the outermost
  message as level 1. A payload nested 16 levels must decode; a payload nested
  17 levels must be rejected as soon as the seventeenth level begins.
- The depth limit applies to every path into a nested message: a
  length-delimited message field, and any element of a repeated message field,
  regardless of whether that element arrived packed or as an individual field.
- The depth counter must be carried down through each recursive descent. It is
  not sufficient to enforce the limit only at the entry point of a decode.

### 2.6 Size limits during decode

- A decode is bounded by a byte budget. The budget applies to all bytes the
  decoder admits, including bytes reached by descending into nested messages,
  not only the outermost buffer. A nested message may not re-open a fresh
  budget and read more than the caller allowed in total.

## 3. Supported field types

| type         | wire type | note                          |
|--------------|-----------|-------------------------------|
| `int32`      | 0         | negative values as 10-byte varint |
| `int64`      | 0         |                               |
| `uint32`     | 0         |                               |
| `uint64`     | 0         |                               |
| `bool`       | 0         |                               |
| `string`     | 2         | UTF-8                         |
| `bytes`      | 2         |                               |
| `message`    | 2         | recurses, see §2.5            |
| `fixed32`    | 5         |                               |
| `fixed64`    | 1         |                               |

A field whose wire type does not match the type declared in the schema is a
protocol error. `int32` and `uint32` values narrower than 64 bits are narrowed
to their declared width once decoded.

## 4. Decoding into an object

### 4.1 Shape

A decoded message is a plain object. Keys are field names from the schema; a
`repeated` field decodes to an array, and a non-repeated field to a single
value. Fields absent from the input are absent from the object.

### 4.2 Unknown and unrecognised fields

- A field the schema does not declare is **unknown**. Unknown fields must be
  preserved verbatim, in the order encountered, attached to the decoded
  object, so that they can be re-emitted byte-for-byte.
- Preservation is required because unknown fields are covered by request
  signatures (§3.3 of `gateway.md`) and a gateway that discards them changes
  the bytes it is asked to authenticate.
- A wire type mismatch on a *known* field number is an error (§3), not an
  unknown field.
- Preserved unknown fields include their original tag and payload encoding;
  re-encoding must not normalise, reorder or re-encode them. Unknown fields are
  re-emitted after the declared fields, in the order they were encountered. A
  message whose input listed its declared fields in ascending field-number
  order and placed its unknown fields after them therefore re-encodes to the
  same bytes.

### 4.3 Errors

Decoding reports a single error type carrying a short machine-readable code and
a human-readable message. A decode either produces a complete object or fails;
it does not return a partially decoded object.

## 5. Encoding

### 5.1 Encoding messages

- `encodeMessage` writes declared fields in ascending field-number order,
  followed by preserved unknown fields in their original order. The result of
  encoding a message that was just decoded must be byte-identical to the
  original input whenever the input contained only fields the schema declares
  and unknown fields (§4.2).
- Repeated fields are written one element per tag (unpacked) by `encodeMessage`,
  except that a repeated scalar field that arrived packed may be re-emitted in
  either accepted form; the bytes must still contain every element exactly
  once.
- A repeated scalar field **accepts both** encodings on decode: the packed form
  (one length-delimited payload holding concatenated scalar values) and the
  individual-tag form. An input may mix the two forms for the same field, in any
  order, and the decoder must produce a single array containing every element
  in wire order.
- A repeated scalar field may be declared `packed` in the schema; that only
  changes the preferred output form and never restricts what input is accepted.

## 6. Round-trip guarantees

For any input that decodes successfully:

1. decoding is deterministic;
2. re-encoding the decoded object reproduces the original bytes when the input
   listed declared fields in ascending field-number order, contained no packed
   repeated scalars, and placed its unknown fields after the declared fields;
3. unknown fields survive both directions unchanged;
4. a decode followed by an encode followed by a decode yields an equal object.
