# Search criteria

This document is normative for translating a structured frontend search into
the arguments of `UID SEARCH`.

## 1. Request shape

A search request carries a criteria object: a JSON object whose keys are
frontend names and whose values are strings, numbers or booleans. The
translation preserves the order in which keys appear.

## 2. Criteria

### 2.1 Allowed keys

Only the keys in the table below may appear in a search request. Frontend key
names are case-insensitive (`From`, `FROM` and `from` are the same key). A `-`
is accepted in place of `_` in key names (`sent-since` equals `sent_since`).

| Frontend key | Wire key | Value |
|---|---|---|
| `from` | `FROM` | text (astring) |
| `to` | `TO` | text (astring) |
| `cc` | `CC` | text (astring) |
| `bcc` | `BCC` | text (astring) |
| `subject` | `SUBJECT` | text (astring) |
| `body` | `BODY` | text (astring) |
| `text` | `TEXT` | text (astring) |
| `since` | `SINCE` | date |
| `before` | `BEFORE` | date |
| `sent_since` | `SENTSINCE` | date |
| `sent_before` | `SENTBEFORE` | date |
| `larger` | `LARGER` | non-negative integer (octets) |
| `smaller` | `SMALLER` | non-negative integer (octets) |
| `unseen` | `UNSEEN` | boolean flag |
| `seen` | `SEEN` | boolean flag |
| `flagged` | `FLAGGED` | boolean flag |
| `answered` | `ANSWERED` | boolean flag |
| `deleted` | `DELETED` | boolean flag |
| `draft` | `DRAFT` | boolean flag |
| `recent` | `RECENT` | boolean flag |
| `uid` | `UID` | uid set (see §3) |
| `charset` | `CHARSET` | see §2.2 |

Any other key is an error. A key MUST NOT be silently ignored: the frontend
believes it filtered on it, so dropping it would return the wrong messages.

A boolean flag with value `true` contributes the bare wire key (`UNSEEN`). With
value `false` it contributes `NOT` followed by the wire key (`NOT UNSEEN`).

A date is given by the frontend as `YYYY-MM-DD` and is written on the wire as
`DD-Mon-YYYY` with an English three-letter month abbreviation, for example
`2024-01-05` becomes `05-Jan-2024`. An impossible date is an error.

### 2.2 CHARSET

`CHARSET` may appear at most once in a request. Its value MUST be exactly
`UTF-8` or `US-ASCII` (the value itself is case-insensitive; it is normalized to
upper case on the wire). A second `CHARSET`, including one that differs only in
the case of the key name, is an error. Any other charset is an error.

When present, `CHARSET` and its value are the first two search arguments:

```
UID SEARCH CHARSET UTF-8 SUBJECT "quarterly report"
```

### 2.3 Free text

The value of every text key is an astring and MUST be encoded according to
`wire-format.md` §2. That means:

* `"` and `\` are escaped;
* CR, LF and NUL cannot appear in a quoted string;
* a value longer than 1024 UTF-8 bytes is written as a literal, with a byte
  count in `{n+}`.

Free text is never concatenated into the command without quoting.

## 3. UID sets

A uid set is a comma-separated list. Each element is either a single uid or a
range `from:to`.

* A uid is an integer in `1..4294967295`.
* `0`, negative numbers, non-numeric text and empty elements are errors.
* `*` denotes the largest possible uid, `4294967295`.
* A range is normalized to ascending order: `5:3` becomes `3:5`.
* Duplicate uids are removed and the result is written in ascending order:
  `1,1,2` becomes `1,2`. Overlapping ranges are merged.
* An empty set is invalid. It MUST NOT be sent as `STORE ""` or as an empty
  argument; the request fails before anything reaches the wire.
