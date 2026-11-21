# iCalendar text values

This document defines how invitefence turns plain text supplied by the meeting
front end into iCalendar content lines. It describes the required behaviour
only; it does not prescribe a particular implementation.

## 1. Scope

The rules below apply to every property whose value type is `TEXT`:

- `SUMMARY` (meeting title)
- `LOCATION`
- `DESCRIPTION`
- `COMMENT`
- the `CN` parameter of an attendee (see `params.md`)

Property names, parameter names, and structured values (`DTSTART`, `DURATION`,
`RRULE`, `UID`) are handled elsewhere and are not escaped as text.

## 2. Output encoding

1. The generated `.ics` is UTF-8.
2. A content line is terminated by CRLF (`\r\n`), never by a bare LF.
3. There is no BOM and no leading blank line.

## 3.1 Escaping TEXT values

Within a TEXT value the following transformations are required, applied to the
input string **before** any folding takes place:

| Input | Output | Meaning in RFC 5545 |
|---|---|---|
| `\` (U+005C) | `\\` | literal backslash |
| `;` (U+003B) | `\;` | literal semicolon |
| `,` (U+002C) | `\,` | literal comma |
| CRLF (`\r\n`) | `\n` | line break, two characters: backslash + `n` |
| LF (`\n`) not preceded by CR | `\n` | line break |
| CR (`\r`) not followed by LF | `\n` | line break |

Notes:

- The slash is escaped first; the escapes introduced for `;`, `,` and line
  breaks must themselves not be re-escaped.
- The two-character sequence `\n` in the output is the *escaped representation*
  of a line break. A literal backslash followed by a literal `n` in the input
  must therefore become `\\n` (escaped backslash, then plain `n`), so that a
  parser does not mistake it for a line break.
- A CRLF in the input produces exactly one `\n` escape, not two. A CRLF must
  never leave a raw CR or LF character in the output.
- After escaping, no raw CR and no raw LF may remain anywhere in a content
  line. Consequently an input value can never terminate the current content
  line or start a new property.
- Escaping must not introduce or remove any byte other than the characters
  shown above; in particular, non-ASCII characters are passed through as UTF-8
  bytes unchanged.

Example: the input display name

```
Alice
ATTENDEE:mailto:mallory@example.com
```

must be serialised as the single token `Alice\nATTENDEE:mailto:mallory@example.com`
inside the `CN` parameter, and must not produce a second `ATTENDEE` property.

## 3.2 Values that are already structured

`DTSTART`, `DTEND`, `DURATION` and `RRULE` are not TEXT values and are not
escaped. They are validated and normalised by their own rules; see `recur.md`.

## 3.3 Empty and missing values

- A missing optional field produces no property at all.
- A present but empty field produces the property with an empty value
  (`SUMMARY:`), which is distinct from omitting it.

## 3.4 Maximum line length

Content lines are limited by the folding rules in §3.5. The limit counts
octets of the **escaped, UTF-8 encoded** value together with the property name
and parameters on the same logical line. It does not count the CRLF at the end
of the physical line.

## 3.5 Folding long lines

1. A logical content line that is at most 75 octets long is emitted as a
   single physical line.
2. A longer logical line is split into several physical lines. The first
   physical line carries at most 75 octets.
3. Each continuation line begins with a single space (U+0020), and that space
   counts toward the 75-octet limit of the continuation line. A continuation
   line therefore carries at most 74 octets of content after the leading
   space.
4. Folding happens only at a boundary between UTF-8 encoded characters. A
   multi-byte UTF-8 sequence must never be split across two physical lines; a
   line is ended at the last complete character that still fits.
5. Folding is measured on the escaped value (§3.1) as encoded in UTF-8, not on
   JavaScript string length and not on UTF-16 code units. Characters outside
   the Basic Multilingual Plane (e.g. emoji) count as the 4 UTF-8 octets they
   occupy.
6. Folding must never occur inside an escape sequence such as `\,`, `\;`,
   `\\`, or `\n`. The escape sequence is treated as part of the value and a
   fold point may fall immediately before or after it, but not between the
   backslash and the escaped character.
7. Folding is a transport concern only: after unfolding (removing CRLF and the
   single leading space of each continuation), the value must be identical to
   the escaped value produced by §3.1.
8. No physical line may exceed 75 octets including the continuation space.
9. A logical line that contains no value beyond the property name (e.g.
   `BEGIN:VEVENT`) is never folded.

Example: a Chinese title whose escaped UTF-8 encoding is 210 octets is emitted
as three physical lines: 75 octets, then a space plus 74 octets, then a space
plus the remaining octets — with every physical line ending in CRLF.

## 3.6 Order of operations

The order is fixed:

1. escape the value (§3.1),
2. append the value to the property name and its parameters with a colon,
3. fold the resulting logical line (§3.5),
4. append CRLF.

Parameters are folded together with the property name and value as one logical
line; the fold point may fall inside the parameter section.