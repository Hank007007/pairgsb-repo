# ClientHello

snisieve reads exactly one message out of the handshake stream: the client's
first ClientHello. This document defines the parts of that message the proxy
relies on, and what counts as malformed. It describes behaviour, not code.

## 1. Scope

Only the first ClientHello of a connection is inspected. Nothing after it is
parsed, and no other handshake message type is interpreted. The parser's job is
to turn the reassembled handshake message (see `docs/record-layer.md` §2.3) into
a small, fixed set of facts: the legacy version, the extension block, and the
server name.

## 2. ClientHello layout

### 2.1 Handshake header

The message begins with a four byte handshake header:

```
msg_type(1) | length(3)
```

`msg_type` must be 1 (client_hello). `length` is big-endian and counts the bytes
after the header. A header that declares more bytes than are present is
malformed. Bytes beyond `4 + length` are not part of this message; they belong
to whatever handshake message follows, and `docs/record-layer.md` §3.1 says what
that means for the connection.

### 2.2 Body fields

After the header, in order:

| field | encoding |
|---|---|
| legacy_version | 2 bytes |
| random | 32 bytes |
| session_id | 1 byte length, then that many bytes |
| cipher_suites | 2 byte length, then that many bytes (even) |
| compression_methods | 1 byte length, then that many bytes |
| extensions | 2 byte length, then that many bytes |

Every length field is big-endian. A length that would run past the end of the
message is malformed. The extension block declared by the `extensions` length
must end exactly where the ClientHello body ends: bytes left over after the
declared block are malformed, and a block that claims more bytes than the body
holds is malformed. The proxy does not silently trim the message to fit.

`legacy_version` is validated. The accepted range is 0x0301 through 0x0303
inclusive. Anything else, including 0x0300 and 0x0304, is malformed. The value
is not used for any other purpose; the negotiated version is irrelevant here.

### 2.3 Extension framing

The extension block is a sequence of extensions:

```
type(2) | length(2) | data(length)
```

Extensions are parsed strictly inside the block declared by §2.2. An extension
whose declared length reaches past the end of the block is malformed. After the
last extension the parser must be exactly at the end of the block; a partial
extension header, or trailing bytes that do not form a complete extension, are
malformed. Extension types the proxy does not understand are skipped without
interpretation, but their framing is still validated.

### 2.4 server_name (type 0)

The SNI extension data is:

```
server_name_list_length(2)
  name_type(1) | host_name_length(2) | host_name
  ...
```

The list length is big-endian and describes the bytes that follow it inside
this extension. It must agree with the extension data: a list length that runs
past the extension, or leaves bytes unused inside it, is malformed. The two
lengths are independent of each other and both must be satisfied.

Constraints on the entry the proxy uses:

* `name_type` must be 0 (host_name). Any other value is malformed; there is no
  fallback interpretation.
* `host_name` is the name the client says it is connecting to. It must be
  non-empty and consist only of printable ASCII bytes (0x21 through 0x7E).
  Bytes outside that range, including NUL, spaces, and any byte with the high
  bit set, are malformed.
* `host_name` may carry a single trailing dot, the fully-qualified form. That
  dot is removed before the name is reported, so `a.example.com.` is reported
  as `a.example.com`. Two trailing dots, a name that is only a dot, and a name
  that becomes empty once the dot is removed are malformed.

The name reported by this layer is otherwise unchanged: same bytes, same case.
Case folding belongs to matching (`docs/policy.md` §3.5), not here.

### 2.5 Other extensions

Extensions other than server_name are not interpreted. Their presence is
recorded only as part of the extension block. A malformed framing anywhere in
the block is fatal even if the malformed extension is not one the proxy uses.

### 2.6 Duplicate server_name

A ClientHello may contain at most one server_name extension. Two server_name
extensions in the same message are malformed, full stop. The proxy must not pick
one of them, must not merge them, and must not prefer the first or the last: the
client and the upstream may disagree about which one is authoritative, and that
disagreement is exactly what this check exists to prevent.

### 2.7 Missing server_name

A ClientHello with no server_name extension carries no name that policy can
match. It cannot be classified and is rejected. It is not treated as a request
for the empty name or for the connection's IP address.

## 3. Result

On success the parser yields the legacy version, the extension block, and the
server name as it appeared on the wire with at most one trailing dot removed
and nothing else changed. Case folding, if any, happens in policy
(`docs/policy.md` §3.5), never here. On any failure in this document the
connection is handled as described in `docs/policy.md` §4.
