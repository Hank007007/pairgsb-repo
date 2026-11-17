# envwrap envelope format

This document specifies the wire format and the rules an implementation must
follow when producing and consuming envelopes. It describes required behavior
only; it does not prescribe an implementation.

The signature provides integrity and authenticity. It provides no
confidentiality: anyone who can read an envelope can read the session it
carries.

## §1. Envelope syntax

An envelope is a single ASCII string:

```
b64url(payload) "." b64url(mac)
```

* There are exactly two segments, separated by one `.` (U+002E).
* Both segments are non-empty.
* `b64url(x)` is the unpadded base64url encoding (RFC 4648 §5) of the raw
  bytes `x`.
* `payload` is the UTF-8 encoded JSON document described in §4.
* `mac` is the 32-byte HMAC-SHA256 digest computed with the shared key over
  the **payload bytes exactly as they appear in the envelope**, i.e. over the
  decoded contents of the first segment and nothing else. Neither the `.`
  separator nor the encoded signature is covered.
* No other characters may appear in the envelope; whitespace around it is not
  part of the format.

The same key and a given session must always produce an envelope that the
receiving side accepts. Two envelopes may differ byte-for-byte as long as both
satisfy this document.

## §2. Verification order

Verification and parsing are separate steps, and verification comes first.

1. Split the envelope into segments and decode the `mac` segment (§3).
2. Recompute HMAC-SHA256 over the raw bytes decoded from the `payload` segment.
3. Compare the two MACs as described in §7.
4. Only after step 3 succeeds may the payload bytes be decoded as text or
   parsed as JSON.

Consequences that must hold:

* A payload that has been altered in any way while retaining its original
  signature is rejected as a signature failure. It is never reported as a JSON
  syntax failure, a schema failure, or an encoding failure, because no parsing
  happens for an unverified payload.
* The MAC is checked against the bytes on the wire, never against a
  re-serialized form of the parsed session. Altering insignificant JSON
  whitespace, member order, or number spelling still invalidates the envelope.
  Conversely, an envelope whose MAC was legitimately computed over a
  non-canonical serialization is accepted.
* The result of opening an envelope is deterministic and independent of the
  order in which a serializer happens to emit members.

## §3. base64url strictness

Both segments must be valid unpadded base64url. Decoding must not repair or
ignore malformed input.

* The alphabet is `A`–`Z`, `a`–`z`, `0`–`9`, `-`, `_`. Every other character,
  including `+`, `/`, `=`, whitespace, and punctuation, invalidates the
  envelope.
* Padding is forbidden. A segment ending in one or more `=` is invalid.
* The length of an unpadded base64url segment is never congruent to 1 modulo 4.
  A segment whose length has that property is invalid, as is a segment that
  cannot be produced by encoding some byte string.
* When the final quantum is partial, the unused low-order bits of the last
  character must be zero. Non-canonical encodings of a byte string are
  invalid.
* A failure in either segment is a format failure (§8), not a signature
  failure, except where §8 says otherwise.

## §4. Payload decoding and JSON

The payload must be a strict JSON document that decodes cleanly.

* The payload bytes are UTF-8. They must decode as UTF-8 in full. A byte
  sequence that is not well-formed UTF-8 invalidates the envelope; substituting
  U+FFFD or any other replacement character is not permitted.
* The text must be exactly one JSON value with no leading or trailing
  non-whitespace content.
* The top-level value must be a JSON object. Arrays, strings, numbers,
  booleans, and `null` are rejected.
* Only the JSON grammar of RFC 8259 is accepted. Trailing commas, comments,
  single-quoted strings, unquoted member names, `NaN`, and `Infinity` are not
  JSON and are rejected.
* Control characters inside strings must be escaped as JSON requires.

## §5. Member names and object reconstruction

The three member names `__proto__`, `constructor`, and `prototype` are
reserved. They never appear on a reconstructed object: at any nesting level,
in any object, a member whose name is one of these is discarded together with
its value before the session is assembled. The remaining members are
reconstructed normally, and the envelope is not rejected merely for carrying a
reserved name.

When a session object is reconstructed:

* It is an ordinary object whose prototype is `Object.prototype`. No member of
  the payload may influence the prototype of the session object or of any
  nested object.
* Every member carried into the session appears as an own enumerable property
  of the corresponding reconstructed object, with its JSON value. No member may
  appear on the prototype chain instead.
* Reading an absent member yields `undefined`. Reading a member such as
  `isAdmin` that was not present in the payload must not yield a value
  inherited from prototype manipulation.
* Reconstruction must not merge unrelated payload members into a shared
  object; each nested object in the payload maps to its own object.
* Arrays are reconstructed as arrays; their elements follow the same rules.

A session whose prototype is not `Object.prototype`, or on which a name that
was absent from the payload resolves to a value, is never a valid result of
opening an envelope.

## §6. Size limits

The following limits apply to an envelope as received. An envelope that
exceeds any of them is rejected.

| Limit | Value | Measurement |
| --- | --- | --- |
| Envelope size | 8192 bytes | UTF-8 byte length of the whole envelope string |
| Nesting depth | 12 | See below |
| Member names | 256 | Total count across the whole payload |

Depth is counted over container values. The top-level payload object is at
depth 1; each object or array nested directly inside a value at depth *n* is at
depth *n* + 1. An envelope whose payload contains a container at depth 13 or
greater is rejected. An empty object at depth 12 is within the limit.

The member-name count is the sum, over all objects in the payload, of the
number of member names each object declares. Member names are counted once per
declaration; array elements are not member names. An envelope with more than
256 declarations in total is rejected.

These checks are performed by the receiving side when an envelope is opened.
`seal` does not itself refuse to serialize a session that would exceed them;
the receiver is the authority on whether an envelope is acceptable.

## §7. Signature comparison

The supplied MAC is compared with the recomputed MAC over the full length of
the digest, in time independent of where the first difference occurs. An
implementation must not use ordinary string or byte equality that returns as
soon as a mismatch is found. A MAC of the wrong length is a signature failure
and must also be handled without leaking its position of difference.

## §8. Errors and keys

Failures are reported through the library's error types, not as bare
`SyntaxError`, `TypeError`, or `RangeError`. Each error carries a stable
`code`:

| Situation | Error | Code |
| --- | --- | --- |
| Malformed envelope, segment, base64url, payload encoding, JSON, or limit exceeded | `EnvelopeFormatError` | `ERR_ENVELOPE_FORMAT` |
| MAC mismatch or unusable MAC | `EnvelopeSignatureError` | `ERR_ENVELOPE_SIGNATURE` |
| No key supplied and `ENVWRAP_SECRET` unset or empty | `EnvelopeConfigError` | `ERR_ENVELOPE_CONFIG` |

The HMAC key comes from the `key` option when one is supplied, otherwise from
the `ENVWRAP_SECRET` environment variable. There is no default key. Keys should
be at least 32 bytes of unpredictable data. Empty keys are a configuration
error.

An invalid key is a configuration problem and is reported as such; it is not a
format problem with the envelope.
