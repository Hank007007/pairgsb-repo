# Wire format

This document is normative for every byte imapquill writes to an upstream IMAP
server. Sections are referenced from the other documents as `wire-format.md §n`.

## 1. Command frames

A command frame is:

```
<tag> SP <command-name> [SP <arguments>] CRLF
```

* `<tag>` is a non-empty ASCII token without spaces, `+`, CR or LF.
* The frame ends with exactly one CRLF.
* Arguments are separated by single spaces.
* A single logical request may produce more than one command frame (for
  example `EXAMINE` followed by `UID SEARCH`). Each frame has its own tag; tags
  of frames that are sent together must be distinct.
* An upstream reply is a line beginning with the same tag and a status word
  `OK`, `NO` or `BAD`.

## 2. Strings (astrings)

An astring carries a user-controlled string: a mailbox name, free text in a
search key, or a body section selector. There are exactly two wire forms: quoted
string and literal.

### 2.1 Choosing quoted string or literal

A value MAY be sent as a quoted string only when all of the following hold:

* it contains no CR (U+000D), no LF (U+000A) and no NUL (U+0000);
* its UTF-8 encoding is at most 1024 bytes long.

Otherwise the value MUST be sent as a literal.

The length limit is measured in UTF-8 bytes, not in Unicode code points. A
900-character Chinese value is 2700 bytes and therefore requires a literal.

If a value cannot be quoted and the caller cannot produce a literal, the
request MUST fail. It MUST NOT be truncated, silently downgraded to a quoted
string, or sent with the offending bytes removed.

### 2.2 Quoted string

```
DQUOTE *QUOTED-CHAR DQUOTE
```

* A `"` inside the value is written as `\"`.
* A backslash inside the value is written as `\\`.
* CR, LF and NUL MUST NOT appear in a quoted string at all; they cannot be
  escaped.
* Every other byte is written as-is (after the value has been encoded for its
  field, e.g. a mailbox name has already been encoded per
  `mailbox-names.md`).

Example: the value `say "hi" \ bye` is written as `"say \"hi\" \\ bye"`.

### 2.3 Literal

A literal is:

```
LBRACE <n> [PLUS] RBRACE CRLF <n bytes>
```

* `<n>` is the number of bytes that follow, in decimal. It is a byte count,
  never a character count.
* imapquill uses non-synchronizing literals: the form is `{n+}`. The server
  does not send a continuation request.
* After the `<n>` bytes, the rest of the command (a space, a closing bracket,
  another argument, ...) follows immediately, and the frame still ends with
  CRLF.
* The literal payload is binary safe: CR, LF and NUL are allowed inside it.

Example, searching for a four-character Chinese subject (12 UTF-8 bytes):

```
A7 UID SEARCH CHARSET UTF-8 SUBJECT {12+}
季度报告
```

### 2.4 Body section selectors in FETCH

The text between `BODY[` and `]` is a user-controlled astring. It follows §2:
empty means the whole message (`BODY[]`); a non-empty selector is quoted or
written as a literal when §2.1 requires it. When a literal is used, the
literal's bytes are the selector and the closing `]` follows the literal
payload.

## 3. Command size limit

The on-the-wire size of one command frame — from the first byte of the tag
through the final LF, including all literal payload bytes — MUST NOT exceed
8192 bytes.

If a frame would exceed the limit, the whole request fails with a local
validation error. A frame MUST NOT be truncated and sent partially.
