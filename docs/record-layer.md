# Record layer

snisieve only ever looks at the first flight of a TLS connection, and it looks at
it as a stream of TLS records. This document states what the record layer is
supposed to do with those bytes. It describes behaviour, not code.

## 1. Scope

The proxy is not a TLS endpoint. It never decrypts, never negotiates, and never
sends anything back. It reads the bytes the client wrote first, cuts them into
records, and hands the handshake payload to the hello parser. Everything here is
therefore about *parsing untrusted bytes safely*.

## 2. Record framing

### 2.1 Record header

Every record starts with a five byte header:

```
type(1) | version(2) | length(2)
```

`length` is the number of payload bytes that follow the header. It is a 16 bit
big-endian field.

Two independent bounds apply to `length`:

* **Protocol bound.** A TLS record payload may not exceed 2^14 bytes (16384).
  A record that declares a longer payload is malformed, regardless of how many
  bytes happen to be buffered.
* **Buffer bound.** A record may only be consumed when the buffer actually
  contains `length` payload bytes after the header. A header that promises more
  bytes than have arrived so far describes an *incomplete* record, not a
  complete one with a short payload. Treating the short tail as if it were the
  whole payload changes what the next layer sees.

Bytes that remain after the last complete record, and that are too few to hold
another header, are an incomplete record header. They must not be silently
dropped: from the proxy's point of view the first flight was not received in a
usable shape.

`version` is carried through but is not used to make an allow decision.

### 2.2 Record types

The type byte values that matter here are the standard TLS ones: change cipher
spec (20), alert (21), handshake (22), application data (23). A ClientHello is
carried in handshake records. Records of other types may precede it in
principle; the record layer reports them faithfully so that the layer above can
decide what they mean.

### 2.3 Handshake reassembly

A handshake message is not tied to a record boundary. One message may be split
across several records, and the split points are chosen by the client, so they
are attacker-controlled.

Reassembly rules:

1. Take handshake records in the order they appear.
2. Concatenate their payloads in that order.
3. Read the handshake message header from the beginning of the concatenation:
   `msg_type(1) | length(3)`, where `length` is big-endian and counts the bytes
   *after* the four byte header.
4. The message is exactly `4 + length` bytes of the concatenation. Bytes beyond
   that point belong to a following handshake message and are not part of this
   one.
5. If the available handshake payload is shorter than `4 + length`, the message
   is incomplete. It cannot be parsed and the connection cannot be classified.

Consequently, a ClientHello delivered in one record, in three records, or in a
record whose payload also contains the start of the next handshake message must
yield the same message bytes at this layer. The parser above must not be able to
tell how the bytes were chopped up.

## 3. Connection-level rules

### 3.1 One ClientHello per connection

The proxy classifies a connection by its first ClientHello. A TLS client that
sends a second handshake message immediately after the ClientHello (a
renegotiation, or any other handshake message in the same first flight) must
have the whole connection rejected. There is no "use the first one and ignore
the rest" mode: the upstream may act on a different message than the proxy
inspected.

This applies whether the extra handshake message arrives as its own record or as
extra bytes inside the record that carried the ClientHello.

## 4. Failure handling

Any condition in this document that is called malformed or incomplete makes the
record layer fail. The proxy's response to a failure is defined in
`docs/policy.md` §4.
