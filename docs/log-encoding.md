# Audit log encoding specification

This document is the contract between the encoder in `internal/encode` and
the SIEM ingest pipeline that parses its output. The encoder emits one record
per audit event in either `key=value` text (`kv`) or JSON (`json`). The
receiver reverses the escaping rules below and reconstructs the original
field values, so every rule here is byte-exact.

## 1. Scope

An audit event consists of a fixed set of core fields plus a set of
request-derived extension fields. Both the core values and the extension
values are untrusted: a User-Agent, a request path, a tenant name, an error
message, or an extension attribute may contain arbitrary Unicode and control
bytes.

The encoder is responsible for:

- collecting the fields of one event (§3),
- normalizing each value (§4),
- escaping the record for the selected format (§5, §6).

The encoder is not responsible for transport framing. It returns the bytes of
one record with no trailing record terminator.

## 2. Record framing

1. One event occupies exactly one physical line.
2. The encoder returns a single record. The record contains no literal
   `CR` (U+000D) or `LF` (U+000A) byte and no literal U+2028 or U+2029.
3. The record contains no other literal byte in the range U+0000–U+001F and
   no literal U+007F (DEL). Those bytes always appear escaped.
4. The transport appends exactly one LF byte after each returned record.
   The encoder itself never emits a trailing LF, CR, or space.
5. A receiver must be able to recover the event boundary from the escaped
   form alone: splitting the stream on LF yields exactly one record per
   event.

Consequently, a value taken verbatim from a request cannot terminate the
record or introduce a second record.

## 3. Fields, names, and ordering

### 3.1 Core fields

The core fields are emitted in this order, exactly once each:

| Position | Name |
|---|---|
| 1 | `timestamp` |
| 2 | `service` |
| 3 | `event_id` |
| 4 | `level` |
| 5 | `tenant_id` |
| 6 | `trace_id` |
| 7 | `actor` |
| 8 | `action` |
| 9 | `resource` |
| 10 | `path` |
| 11 | `user_agent` |
| 12 | `message` |

`timestamp` is rendered in UTC using RFC 3339 with nanosecond precision
(`2006-01-02T15:04:05.999999999Z07:00`), e.g. `2026-03-14T09:00:00Z`.

### 3.2 Extension fields

Extension fields follow the core fields. They are ordered by field name in
lexicographic byte order, not by map iteration order, so that the same event
always produces the same bytes.

### 3.3 Field names

A field name is valid only if it matches:

```
[A-Za-z_][A-Za-z0-9_.-]*
```

An empty name, a name beginning with a digit or punctuation, or a name
containing a space, `=`, `"`, `\`, a control byte, or any other character
outside that set is invalid. Adding an invalid name fails with the
`fields: invalid key` error (§7).

The restriction applies to extension field names even though the JSON
format quotes its keys: the two formats describe the same field set and the
same parser contract.

### 3.4 Reserved names

The core field names in §3.1 are reserved. An extension field whose name is
reserved fails with the `fields: reserved field name` error. It is not
merged, renamed, or allowed to replace the core value, even when the two
values are identical.

### 3.5 Duplicate names

A field name may occur at most once per event. Adding the same name a second
time fails with the `fields: duplicate field name` error. A duplicate is an
error regardless of the values involved and regardless of which source
supplied each occurrence.

## 4. Value normalization

Normalization is applied to every value, core or extension, before escaping.

### 4.1 UTF-8 validity

The output is always valid UTF-8. Each maximal invalid UTF-8 subsequence in
the input is replaced by a single U+FFFD REPLACEMENT CHARACTER. Valid input
is never altered.

### 4.2 Length limit

After §4.1, a value longer than 1024 bytes is truncated to the longest prefix
that is at most 1024 bytes and ends on a UTF-8 rune boundary. Truncation never
splits a multi-byte rune and never produces invalid UTF-8. No ellipsis,
padding, or marker byte is appended.

The limit is in bytes, not runes. A value that is exactly 1024 bytes is not
truncated. Truncation applies after invalid-subsequence replacement, so the
replacement characters are part of the measured length.

### 4.3 No other changes

Values are otherwise preserved byte for byte: no trimming, no case folding,
no whitespace collapsing, and no HTML escaping. An empty value is valid and
is represented explicitly (§5, §6).

## 5. `kv` format

### 5.1 Record layout

A record is a sequence of fields separated by exactly one space (U+0020).
There is no leading or trailing space. Each field is `name=value`.

### 5.2 Bare values

A value is written bare only when it is non-empty and contains none of:

- space (U+0020),
- `"` (U+0022),
- `\` (U+005C),
- `=` (U+003D),
- any byte in U+0000–U+001F,
- U+007F (DEL),
- U+2028 LINE SEPARATOR or U+2029 PARAGRAPH SEPARATOR.

Otherwise the value is written inside double quotes. In particular, the empty
value is written as `""`.

### 5.3 Quoted values

Inside the quotes, the following escapes are used and no other bytes are
modified:

| Input | Output |
|---|---|
| `\` | `\\` |
| `"` | `\"` |
| U+000A (LF) | `\n` |
| U+000D (CR) | `\r` |
| U+0009 (TAB) | `\t` |
| U+2028 | `\u2028` |
| U+2029 | `\u2029` |
| other U+0000–U+001F, U+007F | `\u00XX`, four uppercase hex digits |

All other characters, including `=`, `/`, non-ASCII text, and U+FFFD, are
written literally as UTF-8. The quoted field is closed by an unescaped `"`.

### 5.4 Reversal

A receiver reads a name up to the first `=`, validates the name against
§3.3, then reads the value: if the next byte is `"`, it reads until the next
unescaped `"` and applies the table in §5.3 in reverse; otherwise it reads
until the next space or end of record. Because every delimiter that can occur
in a value is either quoted or escaped, and every escape is unambiguous, the
original normalized value is recovered exactly.

## 6. `json` format

### 6.1 Record layout

A record is a single JSON object. Names appear in the order defined by §3.
No whitespace is emitted between tokens, and the record has no trailing
newline. Example shape:

```json
{"timestamp":"...","service":"...","event_id":"...","level":"...","tenant_id":"...","trace_id":"...","actor":"...","action":"...","resource":"...","path":"...","user_agent":"...","message":"...","ext":"..."}
```

### 6.2 String escaping

Every name and value is a JSON string. Inside a string:

| Input | Output |
|---|---|
| `"` | `\"` |
| `\` | `\\` |
| U+000A (LF) | `\n` |
| U+000D (CR) | `\r` |
| U+0009 (TAB) | `\t` |
| U+0008 (BS) | `\b` |
| U+000C (FF) | `\f` |
| other U+0000–U+001F, U+007F | `\u00XX`, four uppercase hex digits |
| U+2028 | `\u2028` |
| U+2029 | `\u2029` |

`/`, `<`, `>`, `&`, non-ASCII text, and U+FFFD are written literally as
UTF-8. U+2028 and U+2029 are escaped even though they are not JSON control
characters, because common JavaScript consumers treat them as line
terminators.

### 6.3 Conformance

The emitted record is valid JSON per RFC 8259. A conforming parser decodes
each value to exactly the normalized value from §4 (the original value, apart
from invalid-UTF-8 replacement and truncation). Escaping the record must not
change its meaning.

## 7. Errors

Field collection and encoding can fail. On failure the encoder returns a nil
record; no partial record is emitted. The documented errors are:

| Condition | Error |
|---|---|
| Field name violates §3.3 | `fields: invalid key` |
| Extension field uses a reserved name (§3.4) | `fields: reserved field name` |
| Field name already present (§3.5) | `fields: duplicate field name` |
| Unknown output format requested | `encode: unknown output format` |

Callers must treat these as event-rejection errors. They must not drop the
offending field, rename it, or fall back to overwriting another field.
