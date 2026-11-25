# Blob format

This document is normative. It describes the on-the-wire layout of a *chunked
blob*: the byte stream several producer processes write into the shared object
store, and that `blobrestore` reads back.

A blob is produced and consumed on different machines, by different versions of
different programs, and it travels through a shared store that anybody can
write to. Nothing in the stream may be trusted before it has been checked
against this document.

All integers are **little-endian**. `u8`/`u16`/`u32` are unsigned; `i32`/`i64`
are two's complement signed. `f64` is IEEE-754 binary64.

## 1. Overall layout

```
+--------------------------------------------------------------+
| header (14 bytes)                                            |
+--------------------------------------------------------------+
| chunk stream                       [14, type_table_off)      |
+--------------------------------------------------------------+
| type table                        [type_table_off, ref_table_off) |
+--------------------------------------------------------------+
| ref table                         [ref_table_off, len(buf))  |
+--------------------------------------------------------------+
```

The three regions are contiguous and appear in exactly that order. The header
carries the two offsets that let a reader find the tables without parsing the
chunk stream first.

## 2. Header

### 2.1 Field order

| offset | size | field             | type |
|-------:|-----:|-------------------|------|
| 0      | 4    | `magic`           | 4 raw bytes |
| 4      | 2    | `format_version`  | `u16` |
| 6      | 4    | `type_table_off`  | `i32` |
| 10     | 4    | `ref_table_off`   | `i32` |

`magic` is the four ASCII bytes `BLB1`.

### 2.2 Offset validity

`type_table_off` and `ref_table_off` are absolute byte offsets from the start of
the blob. A reader must reject the blob unless

```
0 <= 14 <= type_table_off <= ref_table_off <= len(buf)
```

A negative offset, an offset past the end of the blob, or offsets that do not
respect the order above are all invalid. An offset must never be used to index
the buffer before this check has passed.

### 2.3 Version

`format_version` is the version of *this* document. Only the value `3` is
defined. Any other value — older or newer — describes a layout this reader does
not know and must be rejected before anything else in the stream is interpreted.

### 2.4 Chunk stream region

The chunk stream occupies exactly `[14, type_table_off)`. A blob whose stream
ends in the middle of a chunk is invalid.

### 2.5 Type names and the allowlist

The type table may only name these types:

```
builtins.list
builtins.dict
builtins.tuple
builtins.str
builtins.int
builtins.float
builtins.bool
builtins.NoneType
```

The `ext` section may only name `__bytes__`.

Every other name is rejected, whether or not it happens to be importable from
the interpreter's `builtins` module. The allowlist is exhaustive and is not a
syntactic filter: `eval`, `exec`, `__import__`, `os.system`,
`builtins.open` and `builtins.list.__class__` are all rejected even though they
are ordinary attribute paths or ordinary builtins.

The allowlist applies to the whole table, not only to the entries that the
chunk stream happens to reference.

## 3. Values

### 3.1 Values, records, refs

The chunk stream is a sequence of **records**. A record is one chunk; the chunk
payload holds exactly one **value**. Records are numbered from 0 in stream
order.

The **root** of the object graph is the *last* record in the stream. Every other
record exists only to be referenced.

Record *i* is addressable as ref id *i* — see §4.2. A value of kind `REF` may
only point at a record that comes **earlier in the stream** than the record
that contains the `REF`. A `REF` that points at a later record (a *forward
reference*), at the record it is currently being decoded from, or at an id
past the ref table, is invalid regardless of whether the pointed-at bytes
would eventually make sense. A reader may therefore assume, when it reaches a
`REF`, that every record it can legally name has already been read in full.

### 3.2 Chunk framing

```
chunk ::= length:u32  payload:length bytes
```

`length` is the number of **payload** bytes that follow it. The payload always
starts with a one-byte `kind`:

| kind | meaning |
|-----:|---------|
| 0x01 | `VALUE` — a single value follows |

Other kinds are reserved and must be rejected.

A chunk is only valid if its whole payload lies inside the chunk stream region.
A declared `length` larger than the number of bytes remaining before
`type_table_off` means the blob is truncated and must be rejected; a reader must
not hand back the shorter prefix that happens to be present, and must not
allocate memory for the declared size before that check has passed.

### 3.3 Value encoding

A value is a one-byte tag followed by its operand:

| tag  | name    | operand |
|-----:|---------|---------|
| 0x00 | `NONE`  | — |
| 0x01 | `BOOL`  | `u8`, must be 0 or 1 |
| 0x02 | `INT`   | `i64` |
| 0x03 | `FLOAT` | `f64` |
| 0x04 | `STR`   | `u32` byte length, then that many UTF-8 bytes |
| 0x05 | `TYPE`  | `u16` type id, `u16` count, then the children (see below) |
| 0x06 | `REF`   | `u16` ref id |
| 0x07 | `EXT`   | `u8` name length, name bytes, `u32` payload length, payload bytes |

A value must not run past the end of its chunk payload.

`TYPE` children:

* `builtins.list`, `builtins.tuple` — `count` values, in element order.
* `builtins.dict` — `count` **entries**, each entry a key value followed by a
  value value. Keys must be unique as Python values; a repeated key is invalid.
* `builtins.str`, `builtins.int`, `builtins.float`, `builtins.bool`,
  `builtins.NoneType` — `count` must be 0; the resulting value is the type's
  empty value (`""`, `0`, `0.0`, `False`, `None`).

`EXT` names must be `__bytes__`. An `EXT` value carries an opaque byte payload.

### 3.4 Shared references

The same ref id denotes one object. Within one restored graph, two occurrences
of the same ref id must yield objects that are `is`-identical, and the pointed-at
record must be decoded once. A graph that mentions a record *n* times must not
cost *n* times the work of decoding it; a structure that references the
previous record twice at each of 20 levels has 21 records and must produce a
graph with a node count linear in the number of levels, not exponential.

### 3.5 Nesting depth

Nesting depth counts **containers**: the root value has depth 1 if it is a
`TYPE` value, and each `TYPE` value that appears directly as a child of another
`TYPE` value is one level deeper than its parent. Scalar values (`NONE`, `BOOL`,
`INT`, `FLOAT`, `STR`) do not add a level. A `REF` does not add a level of its
own — the referent keeps whatever depth it has as a record of its own.

A blob that nests **more than 64** `TYPE` values is invalid. The limit is
enforced while the graph is being built, at every level and however the nesting
is reached — inline children and ref-followed records both count, and the count
does not restart when a `REF` is followed.

## 4. Tables

### 4.1 Type table

At `type_table_off`:

```
count   : u16
entry*  : type_id:u16, name_len:u8, name:name_len bytes (UTF-8)
```

`count` entries follow. The table ends exactly at `ref_table_off`; leftover
bytes, a truncated entry, or an entry that runs past `ref_table_off` all make
the blob invalid. Names are validated against §2.5 when the table is loaded.

A **type id** that appears twice in the table is invalid, even if both entries
name the same type. The table is not a "last one wins" mapping.

### 4.2 Ref table

At `ref_table_off`:

```
count   : u16
offset* : i32, one per ref id, in ascending ref-id order
```

Each offset is the absolute position of a chunk header inside the chunk stream.
Ref ids start at 0, so the entry at index *i* describes ref id *i*. An offset
that does not point at a chunk boundary inside `[14, type_table_off)`, or a
`REF` whose id is `>= count`, makes the blob invalid.

### 4.3 Ext section

Ext values are encoded inline as `EXT` values (§3.3). There is no separate ext
table in this version; `EXT` is listed here because third-party producers
negotiate the ext names they are allowed to use, and `__bytes__` is the only
name this version accepts.

### 4.4 Canonical re-encoding

The blob format admits many byte streams for one object graph. For cache keys
the service needs one *canonical* byte stream per graph:

* two graphs that are equal as values must re-encode to byte-identical blobs,
  no matter in which order the producer inserted keys or built the nodes;
* objects that are the same object (`is`) in the graph must stay shared in the
  re-encoded blob — written once as a record and referenced afterwards;
* values carrying an `EXT` payload (`bytes(value)` is meaningful) must keep that
  payload;
* the byte stream must not depend on address-space layout, hash seeds, or the
  history of the process that produced it.

## 5. Worked example

A blob holding the list `["a", 1]`, 59 bytes in total:

```
offset  bytes                                  meaning
0       42 4c 42 31                            "BLB1"
4       03 00                                  format_version 3
6       27 00 00 00                            type_table_off = 39
10      39 00 00 00                            ref_table_off  = 57
14      15 00 00 00                            chunk length = 21
18      01                                     kind VALUE
19      05                                     tag TYPE
20      00 00                                  type id 0
22      02 00                                  count 2
24      04                                     tag STR
25      01 00 00 00                            byte length 1
29      61                                     "a"
30      02                                     tag INT
31      01 00 00 00 00 00 00 00                value 1
39      01 00                                  type table count 1
41      00 00 0d 62 75 69 6c 74 69 6e 73 2e 6c 69 73 74
                                               id 0, len 13, "builtins.list"
57      00 00                                  ref table count 0
```

The chunk payload length is 21 = 1 kind byte + 20 value bytes. The chunk
stream spans `[14, 39)`, the type table `[39, 57)`, and the ref table
`[57, 59)`.

## 6. Error reporting

Anything this document calls "invalid" must be reported as an error to the
caller. A reader must fail closed: it must not substitute a default type, skip
an unreadable entry, or return a partially decoded graph. On failure no object
graph is handed back.
