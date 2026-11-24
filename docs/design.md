# Design

This document describes what imapquill is responsible for and what "correct"
means at each layer. It contains no implementation; the normative rules live in
`wire-format.md`, `search-criteria.md` and `mailbox-names.md`.

## 1. Why the proxy exists

The frontend does not speak IMAP. It sends small structured requests:

```json
{"op": "search", "folder": "收件箱 & 归档", "criteria": {"subject": "季度报告"}}
{"op": "fetch",  "folder": "INBOX", "uids": "1:10"}
{"op": "store",  "folder": "INBOX", "uids": "1,2", "flags": ["\\Seen"]}
```

Every value in those requests is user-controlled: folder names, search text,
labels. imapquill is the only component between that data and the socket, so it
is the only place where quoting, encoding and framing can be got right.

The pipeline is always the same:

```
frontend request
  -> normalize        (decode/normalize names, validate criteria, uid sets)
  -> render           (astring, mailbox encoding, literal)
  -> frame            (tag + command + args + CRLF, size check)
  -> send             (write frames upstream, collect replies)
  -> classify         (ok / local error / upstream NO / upstream BAD)
```

Each stage has one owner. A rule must be enforced in the stage that owns it,
not duplicated ad hoc in a caller.

## 2. Layers and responsibilities

### 2.1 `proto` — the wire

Owns the byte-level representation.

* **astring.** A value is a quoted string only if it contains no CR, LF or NUL
  and its UTF-8 encoding is at most 1024 bytes. Otherwise it is a literal.
  Quoted strings escape `"` and `\`. Literals declare their byte count in
  `{n+}`, are followed by CRLF, then exactly `n` bytes, then the rest of the
  command and the final CRLF.
* **framing.** A frame is `<tag> SP <command> [SP <arg>]* CRLF`; its UTF-8 size,
  literal payloads included, may not exceed 8192 bytes. An over-long frame is a
  local error, never a truncated frame.
* **mailbox names.** User-visible names cross the wire in modified UTF-7:
  printable ASCII except `&` is literal, `&` becomes `&-`, and every run of
  non-ASCII is a `&<base64>-` shifted sequence. Decoding is the inverse and
  must handle base64 padding and report malformed sequences as a local error.

The layer must be total in the sense that every string that reaches it either
renders to a valid wire form or raises a clearly typed local error. It must
never emit a bare CR, LF or NUL inside a command frame.

### 2.2 `search` — criteria and uid sets

Owns the translation of frontend filters.

* The key set is exactly the table in `search-criteria.md §1`. Key names are
  case-insensitive and `-`/`_` are interchangeable. Unknown keys are errors.
* Text values are astrings; the layer hands them to `proto` rather than
  concatenating. Dates and integers are validated and rendered per the table.
* `CHARSET` appears at most once, and only `UTF-8` or `US-ASCII`.
* A uid set is validated: positive integers only, `*` means the maximum uid,
  ranges ascend, duplicates and overlaps collapsed, output ascending. An empty
  set is an error, not an empty argument.

### 2.3 `cmd` — operations to commands

Owns the shape of each operation.

* `search` -> `EXAMINE` the (encoded) folder, then `UID SEARCH` with the
  rendered criteria.
* `fetch` -> `EXAMINE`, then `UID FETCH <uidset> BODY[<section>]`, where the
  section is rendered as an astring per `wire-format.md §2.4`.
* `store` -> `EXAMINE`, then `UID STORE <uidset> +FLAGS (<flags>)`. An empty
  uid set never reaches the wire.

Builders validate what they can before framing; a builder either returns every
frame of the request or raises.

### 2.4 `app` — session and dispatch

Owns identity, caching and result classification.

* **Folder identity.** The cache key is derived from the decoded mailbox name,
  with `INBOX` canonicalized case-insensitively. All other names are
  case-sensitive. Parent/child paths use the delimiter announced by `LIST`,
  not a hard-coded `/`.
* **Dispatch.** Local failures (bad parameters, values that cannot be
  represented) and upstream rejections (`NO`, `BAD`) are different outcomes and
  reach the frontend as different result codes. Logs identify which class
  occurred and include the operation and the failing value's context.
* **Fail closed.** If any frame of a request cannot be built, nothing of that
  request is sent.

## 3. Result codes

| situation | meaning |
|---|---|
| `ok` | every frame was accepted by the upstream |
| `bad_request` | the frontend request is invalid; nothing sent |
| `protocol_error` | a value cannot be represented; nothing sent |
| `upstream_no` | upstream answered `NO` |
| `upstream_bad` | upstream answered `BAD` |

The distinction matters: `bad_request` tells the frontend to fix its payload,
while `upstream_no` tells it the mailbox or selection does not exist. Treating
them alike hides upstream failures and makes the wrong party retry.

## 4. Examples of correct end-to-end behaviour

These are behaviours, not outputs to copy.

1. A label containing `"...\r\nLOGOUT"` produces exactly one command frame.
   The CR/LF cannot be represented in a quoted string, so the value is a
   literal whose declared byte count matches the payload, and the command
   continues after it.
2. A 900-character Chinese subject is 2700 UTF-8 bytes. It is a literal with
   `{2700+}`, not a quoted string.
3. `收件箱 & 归档` encodes so that the `&` between words is `&-`, and decoding
   the result returns the original string.
4. `INBOX` and `inbox` map to one cache entry; `Archive` and `archive` map to
   two.
5. `5:3` renders as `3:5`, `1,1,2` as `1,2`, `0` and `-1` are rejected, `*`
   becomes `4294967295`.
6. An empty selection is a local error; `UID STORE ""` is never written.
7. A repeat or unsupported `CHARSET` is a local error; the `CHARSET` argument,
   when present, is the first two search arguments.
