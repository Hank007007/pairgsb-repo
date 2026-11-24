# Mailbox names

This document is normative for how user-visible mailbox names cross the wire
and how they are cached.

## 1. Encoding

Mailbox names on the wire use the modified UTF-7 encoding defined by RFC 3501
§5.1.3.

* Printable US-ASCII in the range `0x20..0x7e` is copied unchanged, **except**
  `&`.
* A literal ampersand is encoded as `&-`. This is mandatory: `&` starts a
  modified-UTF-7 shifted sequence, so an unencoded `&` makes the name
  undecodable.
* A run of non-ASCII characters is encoded as `&`, followed by the base64
  encoding of the UTF-16BE code units, followed by `-`. In that base64
  alphabet `/` is replaced by `,` and `=` padding is omitted.
* Decoding reverses this. Base64 padding must be restored before decoding; an
  unterminated or non-base64 shifted sequence is reported as a clear local
  error, not as a raw `binascii.Error`.

Example: `收件箱 & 归档` encodes to a string in which the spaces stay spaces,
each Chinese run becomes a `&...-` sequence, and the `&` between the words
becomes `&-`. Decoding that wire name returns exactly `收件箱 & 归档`.

## 2. INBOX

`INBOX` is case-insensitive: `INBOX`, `inbox`, `Inbox` and any other casing
name the same mailbox. The canonical external spelling is `INBOX`, and caches
and comparisons use that canonical spelling.

Every other mailbox name is case-sensitive: `Archive` and `archive` are two
different mailboxes and must never share a cache entry.

## 3. Hierarchy delimiter

The hierarchy delimiter is a property of the server and is announced by the
`LIST` response (`(\HasChildren) "/" "Archive"` announces `/`, other servers
may announce `.` or `\`). When a parent name and a child name are joined, the
delimiter from `LIST` is used.

The delimiter MUST NOT be hard-coded to `/`. A server whose delimiter is `.`
expects `Archive.2024`, not `Archive/2024`.
