# PATCH /documents/{id} — patch policy

This document specifies the behavior the patch layer must provide. It describes
the contract only; it does not prescribe an implementation.

## §1 Scope and trust boundary

The layer receives the current document, the patch operation sequence submitted
by the client, and a `Principal{Role, TenantID}`. Authentication (JWT) and audit
logging happen outside this layer. The layer must therefore enforce, on its own,
that:

- every operation targets only the document being patched;
- every write lands on a field the principal is allowed to modify;
- the operation sequence is applied atomically: either every operation succeeds
  and the resulting document is written, or the document is left byte-for-byte
  unchanged.

Document mutation is all-or-nothing. An error code is returned together with the
unmodified input document.

## §2 JSON Pointer

Paths are JSON Pointers as defined by RFC 6901.

- `""` addresses the whole document.
- A pointer that does not begin with `/` is malformed.
- A pointer beginning with `/` is a sequence of reference tokens, each prefixed
  by `/`. `"/"` addresses the member whose key is the empty string.
- Inside a token, `~1` denotes `/` and `~0` denotes `~`. Escaping is decoded in
  the order `~1` first, then `~0`, so that `~01` denotes the literal `~1`.
- A `~` that is not followed by `0` or `1` is a malformed escape. This includes
  a `~` at the end of a token. Such a pointer is rejected with
  `invalid-pointer`.

Object traversal uses exact key matching after escape decoding. Array traversal
uses a decimal index without sign, leading zeros are not permitted, and the index
must be less than the array length. The token `-` is not an index and is only
meaningful as the target of an `add` operation on an array (append position). A
malformed or negative array index yields `invalid-index`; an in-range-syntax
index that is larger than the array yields `index-out-of-range`.

When a path cannot be traversed because an intermediate value is `null`, a
string, a number, or a boolean, the result is `path-not-container`. A missing
object member or an unusable array position yields `path-not-found`. A path is
never resolved to a placeholder or zero value; failure is reported.

The maximum pointer depth (number of reference tokens) is given in §6.

## §3 Operations

Exactly six operations are supported: `add`, `remove`, `replace`, `move`,
`copy`, `test`. Any other `op` value yields `unknown-op`. Required members:

| op | required |
|---|---|
| add | `path`, `value` |
| remove | `path` |
| replace | `path`, `value` |
| move | `from`, `path` |
| copy | `from`, `path` |
| test | `path`, `value` |

A missing required member yields `missing-member`. A missing or malformed
`path`/`from` yields `invalid-pointer`.

`value` may be any JSON value, including `null`.

### §3.1 add

- Object target: the member is created if absent and overwritten if present.
- Array target: the token must be a valid index in `0..len`, or `-`. The value
  is inserted at that position, shifting later elements right. `-` appends. An
  index greater than `len` yields `index-out-of-range`; a negative or malformed
  index yields `invalid-index`.
- The whole-document pointer is not a valid write target.

### §3.2 remove

The addressed member or element must exist. Removing an object member deletes
it; removing an array element shifts later elements left. A missing target
yields `path-not-found`; this operation never creates anything.

### §3.3 replace

The addressed member or element must already exist and is replaced in place. A
missing target yields `path-not-found`.

### §3.4 move

`from` must exist. The value is removed from `from` and inserted at `path`
using `add` semantics. Removal happens before insertion, so indices in `path`
are interpreted against the document after removal.

If `from` is a proper prefix of `path` (target is the source itself or a
descendant of it), the operation yields `invalid-move`. Moving a location onto
itself is permitted and leaves the document unchanged.

### §3.5 copy

`from` must exist. The value is inserted at `path` using `add` semantics. The
inserted value must be an independent deep copy: later changes to either
location must not be visible at the other.

### §3.6 test

The value at `path` is compared with `value` for deep equality. Comparison is
structural: objects compare by member set regardless of key order, arrays
compare element by element in order, and numbers compare by numeric value, so
`1`, `1.0`, and `1e0` are equal. Types other than numbers must match. A mismatch
yields `test-failed`. A path that cannot be resolved yields the resolution
error from §2.

## §4 Field authorization

Authorization is evaluated for every operation before any operation is applied.
`test` does not write and is not subject to field authorization. For `move` and
`copy`, both `from` and `path` must be writable; either one being read-only
rejects the operation with `field-read-only`.

Write access is decided by the first reference token (the top-level field). The
whole-document pointer is never writable. Nested paths inherit the permission of
their top-level field, so `/tags/0` is governed by `tags`.

The following fields are read-only for every role: `id`, `tenant_id`,
`owner_id`, `role`, `acl`, `created_at`, `updated_at`. Attempting to write any
of them — including a nested path such as `/acl/0` — yields `field-read-only`.

For all other top-level fields the role determines access:

| field | viewer | editor | admin |
|---|---|---|---|
| title | no | yes | yes |
| summary | no | yes | yes |
| body | no | yes | yes |
| tags | no | yes | yes |
| metadata | no | yes | yes |
| status | no | no | yes |
| anything else | no | no | no |

An unrecognized role has no write access.

## §5 Atomicity

The input document is never modified. If any operation fails, the caller
receives the original document unchanged together with the error. No prefix of
the operation sequence may be observable, including when the failure comes from
`test`.

## §6 Limits

Checked before any operation is applied. A violation yields `limit-exceeded`.

| limit | value |
|---|---|
| operations per request | 64 |
| reference tokens per pointer (`path` and `from`) | 16 |
| length of any single string value being written | 4096 characters |

String values are checked recursively inside objects and arrays.

## §7 Error codes

| code | meaning |
|---|---|
| `invalid-pointer` | malformed pointer or escape |
| `path-not-found` | target member/element does not exist |
| `path-not-container` | traversal reached `null` or a scalar |
| `invalid-index` | array index malformed or negative |
| `index-out-of-range` | array index beyond the array |
| `unknown-op` | unsupported `op` |
| `missing-member` | required operation member absent |
| `invalid-move` | move target is a descendant of the source |
| `field-read-only` | principal may not write the target |
| `test-failed` | `test` value does not match |
| `limit-exceeded` | request limit violated |

## §8 Demo data

`cmd/patchdemo` reads `testdata/scenarios.json`. Each scenario names a document,
a patch operation sequence, and a principal, and prints the document before and
after (or the error code if the sequence is rejected).
