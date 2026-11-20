# DER / client-certificate policy

This document is the normative description of what the identity service must
accept. It states observable behaviour only; it does not prescribe how the
decoder is organised.

The gateway forwards the raw bytes of a peer certificate, with no framing and
no guarantee that those bytes are well formed. A certificate is accepted only
when **all** rules below hold. Every rejection must raise a typed error whose
name identifies the rule family.

## §1 Length encoding

1. A length occupies one or more octets. The short form is a single octet in
   `0x00`–`0x7F`.
2. The long form starts with `0x81`–`0xFE`; the low seven bits are the number
   of subsequent length octets. That count is at least one and at most 126.
3. Indefinite length (`0x80`) is a BER construction. It is never accepted;
   reject with `LengthError`.
4. Length octets must use the shortest possible encoding:
   - a long-form length whose first content octet is `0x00` is not minimal;
   - a value below `0x80` written in long form (`0x81 0x05`) is not minimal;
   - `0x80` as a count (`0xFF`) and a count larger than needed are invalid.
   Each of these is rejected with `LengthError`.
5. A length must not exceed the number of content octets actually remaining
   after the header (or, for an outer element, the number of bytes supplied).
   An overrun is rejected with `TruncatedError`; it must not be clamped to the
   available bytes.
6. Lengths are unsigned and are decoded without sign extension. Values above
   `0x7FFFFFFF` may not be represented by the reader's bounded representation
   and must be rejected with `LengthError` rather than wrapped.
7. After the outer certificate element there must be no trailing octets.
   Leftover input is rejected with `TruncatedError` (or `LengthError` when the
   leftover begins inside the length field). Silently ignoring a tail is not
   acceptable.

## §2 Structure, nesting, and resource limits

1. Tags are decoded per X.690: class bits, constructed bit, and low tag
   number; the high-tag-number form is used when the low five bits are all
   ones.
2. Only the constructed form carries nested elements. A primitive element's
   content is opaque to the structural reader.
3. Nesting is limited to **32** levels. A document that nests deeper is
   rejected with `DepthError`. The limit applies to arbitrary input; a deeply
   nested document must not be able to exhaust the interpreter's call stack.
4. A single document may contain at most **4096** elements. Every element
   counts — the outer certificate, `tbsCertificate`, every extension wrapper,
   every `GeneralName`, every nested value — not only the top-level element.
   Exceeding the budget is rejected with `ElementCountError`.
5. No single element may be larger than **1 MiB**. This applies to any element,
   nested ones included, and is rejected with `ElementSizeError`.
6. Budget state belongs to exactly one certificate decode. The service may
   reuse a `Budget` object between requests, but every decode begins from a
   zeroed state, including a decode that follows one that failed.

## §3 INTEGER

1. INTEGER is two's complement, big-endian.
2. A DER INTEGER uses the minimum number of content octets:
   - positive values must not begin with `0x00` followed by a byte below
     `0x80`;
   - negative values must not begin with `0xFF` followed by a byte at or above
     `0x80`.
   A non-minimal INTEGER is rejected with `LengthError`.
3. An empty INTEGER is rejected with `LengthError`.

## §4 Serial number

1. The serial number is an INTEGER and therefore obeys §3.
2. It must be positive: a value whose sign bit is set is rejected with
   `CertificateError`.
3. Its content must be at most **20** octets. Longer values are rejected with
   `CertificateError`.

## §5 OBJECT IDENTIFIER

1. Content octets encode base-128 arcs; the high bit marks continuation.
2. Every arc, including the first combined arc, must be below `2**32`.
   A larger arc is rejected with `OIDError`.
3. The first combined arc encodes the first two components: component one is
   `arc // 40` and component two is `arc % 40`, with the constraint that the
   first component is `0`, `1`, or `2`, and when it is `2` the second component
   may exceed `39`. An encoded first arc that would yield a first component
   above `2` is rejected with `OIDError`.
4. An OID needs at least two components, so its content is never empty.
5. The final arc must be terminated (high bit clear). An unterminated tail is
   rejected with `OIDError`; it is never silently dropped.

## §6 Time

1. `UTCTime` has the form `YYMMDDHHMMSSZ`. Years `00`–`49` map to 1950–1999;
   years `50`–`99` map to 2000–2049. Seconds are mandatory and there is no
   fractional part.
2. `GeneralizedTime` has the form `YYYYMMDDHHMMSS`, followed by either `Z`, a
   signed `±hhmm` offset, or nothing.
3. `Z` and a numeric offset both denote UTC instants. `20240101020000+0200`
   and `20240101000000Z` must produce the same instant. A missing zone designator
   is an error in a certificate, not a local-time fallback; it is rejected with
   `TimeError`.
4. A fractional part is decimal digits after a dot, representing a fraction of
   a **second** (`.5` is 500 milliseconds), not of any larger unit. It may only
   follow the seconds field and must be present only when the zone form allows
   it.
5. Invalid calendar values (month 13, hour 24, and so on) are rejected with
   `TimeError`.

## §7 Certificate structure and validity

1. A certificate is `SEQUENCE { tbsCertificate, signatureAlgorithm,
   signatureValue }` with no fourth element.
2. `version` is `[0] EXPLICIT` and defaults to v1. Explicitly encoded versions
   are 0, 1, or 2; other values are rejected with `CertificateError`.
3. The `tbsCertificate` must be retained as its exact original bytes, from its
   tag octet through its final content octet, so that the signature can be
   checked over precisely those bytes. A re-encoded or reconstructed copy is
   not acceptable.
4. The outer `signatureAlgorithm` must equal the `signatureAlgorithm` inside
   `tbsCertificate`; the comparison covers the algorithm OID and its
   parameters. A mismatch is rejected with `CertificateError`.
5. The two validity times follow §6 and must satisfy `notBefore <= notAfter`.
   An inverted interval is rejected with `CertificateError`.
6. Names are decoded from RDN sequences; a malformed name is rejected with
   `CertificateError`.

## §8 Extensions

1. Extensions are a `[3] EXPLICIT` wrapper around `SEQUENCE OF Extension`.
2. An extension OID may appear at most once. A duplicate is rejected with
   `CertificateError`; a later value must not replace an earlier one.
3. An extension the service understands is decoded and its parsed form is
   exposed. An extension the service does not understand is retained as opaque
   bytes.
4. If an unrecognised extension is marked `critical`, the certificate is
   rejected with `CertificateError`. Unknown non-critical extensions are
   benign.
5. `subjectAltName` is `SEQUENCE OF GeneralName`. Each `GeneralName` must be a
   context-specific tag; only `dNSName` (`[2]`) contributes DNS identities.
   Other GeneralName kinds are not DNS names and must not be returned as
   identities.
6. A `dNSName` must be valid IA5/UTF-8 text with no NUL octet. Entries
   containing a NUL or undecodable bytes are rejected; they never reach the
   identity list.
7. `basicConstraints`, `keyUsage`, `extKeyUsage`, `subjectKeyIdentifier`, and
   `authorityKeyIdentifier` decode as described by RFC 5280.

## §9 Identity selection

1. When a certificate has a `subjectAltName` containing at least one
   `dNSName`, that SAN is the identity source. Only when no SAN DNS name exists
   may the subject's common name be used.
2. If neither exists, no identity is produced.
3. DNS comparison is case-insensitive.
4. Names are compared after IDNA (A-label) conversion and Unicode NFC
   normalisation, so `ＥＸＡＭＰＬＥ.com` and `example.com` denote the same
   name, and a full-width or decomposed spelling cannot be used to obtain a
   distinct identity.
5. A wildcard `*.example.com` matches exactly one label in that position:
   `a.example.com` matches; `a.b.example.com` does not; the bare
   `example.com` does not; and `example.com.attacker.net` never matches.
   The wildcard must be the entire left-most label.

## §10 Chain validation

1. The chain starts at the supplied leaf and proceeds issuer by issuer.
2. A candidate issuer is selected by name **and** key identifier: the child's
   `authorityKeyIdentifier.keyIdentifier`, when present, must equal the
   candidate's `subjectKeyIdentifier`. The child's issuer name must equal the
   candidate's subject name. When several candidates share a name, the key
   identifier decides; taking the first name match is not acceptable.
3. The chain must terminate at a certificate supplied in the trust-anchor set.
   A self-issued certificate that is not in the trust-anchor set does not
   terminate the chain; a self-signed leaf therefore does not authenticate
   itself.
4. Every issuing certificate — every certificate in the chain except the last,
   plus the anchor when it issues — must have `basicConstraints` with
   `CA:TRUE`. A missing or false `CA` is rejected with `ChainError`, naming the
   offending certificate.
5. When `keyUsage` is present on an issuing certificate it must assert
   `keyCertSign`.
6. `pathLenConstraint` limits the number of non-self-issued intermediate
   certificates that may follow the certificate in the chain. A certificate
   whose `pathLenConstraint` is smaller than the number of subordinate
   certificates present is rejected with `ChainError`, naming the offending
   certificate.
7. When client authentication is required, the leaf's `extKeyUsage`, if
   present, must contain `id-kp-clientAuth` (`1.3.6.1.5.5.7.3.2`). A chain
   lacking it is rejected with `ChainError`.
8. Chain building must detect loops and must not accept a chain merely because
   a name lookup succeeded.

## §11 Errors

| Condition | Error |
|---|---|
| length not minimal / not representable | `LengthError` |
| input ends inside a header or element content | `TruncatedError` |
| indefinite length | `LengthError` |
| nesting above 32 | `DepthError` |
| more than 4096 elements | `ElementCountError` |
| element above 1 MiB | `ElementSizeError` |
| malformed OID | `OIDError` |
| malformed time | `TimeError` |
| malformed tag or unexpected tag | `TagError` |
| structurally valid DER that is not an acceptable certificate | `CertificateError` |
| unacceptable chain | `ChainError` |

All of these derive from `ValueError`.
