# Cookie Envelope v2 — Wire Format and Acceptance Rules

Status: normative (ratified by Security & Authentication)
Applies to: every service that calls `issue` / `read` from `cookieveil`
Supersedes: v1 envelopes (out of scope for this repository)

This document defines the external contract of a sealed cookie. It describes
observable behaviour only. It is the authority whenever code, tests, or
comments disagree with it.

Terms used below:

- **cookie** — the opaque string written to the client and read back by
  internal services.
- **claims** — the JSON object recovered from a verified cookie.
- **key ring** — the set of symmetric keys known to a service, each with a
  `kid` and a validity window.
- **reader** — any service calling `read`.
- **issuer** — the login service calling `issue`.

## §1 Format overview

A v2 cookie is exactly four dot-separated segments:

```
v2.<header>.<body>.<mac>
```

- Segment 1 is the literal ASCII string `v2`.
- Segment 2 (`header`) is the base64url encoding of a JSON object.
- Segment 3 (`body`) is the base64url encoding of a raw-deflate compressed
  JSON object.
- Segment 4 (`mac`) is the base64url encoding of a 32-byte HMAC-SHA256 tag.

All encoded segments use the URL-safe alphabet (`A-Z`, `a-z`, `0-9`, `-`, `_`)
**without** padding. The empty string is not a valid encoding of any segment.

The cookie is ASCII. A cookie that is not ASCII, or that contains whitespace,
must be rejected; readers must not trim, lowercase, or otherwise normalise the
input before parsing.

## §2 Canonical segmentation

§2.1 A cookie contains exactly four segments. A string with fewer or more
than four segments is invalid, even if the extra segments could be ignored.

§2.2 Decoding a segment is strict. The reader accepts only characters from
the base64url alphabet, only the canonical unpadded encoding of the original
bytes, and only a length consistent with unpadded base64url. Any other
character, padding, or non-zero trailing bits makes the cookie invalid.

§2.3 Exactly one string represents a given (header, body, mac) triple. Two
different cookie strings must never decode to the same segments, and no
segment may decode to bytes that would re-encode to a different string.

§2.4 Readers must fail closed: an invalid encoding is a rejection, never a
truncation, skip, or best-effort decode.

## §3 Header

The header is a JSON object with at least:

- `kid` (string, non-empty) — identifier of the key used to produce the MAC.
- `comp` (string) — body compression algorithm. Version 2 defines exactly one
  value, `deflate-raw`. Any other value is rejected by readers.

Additional header members are permitted. Header members are authenticated as
part of the envelope (§4), and any change to the header invalidates the MAC.
Readers must reject a header that is not a JSON object, that is missing
`kid` or `comp`, that carries a `kid` of the wrong type, or whose decoded size
exceeds the envelope limits in §8.

## §4 Message authentication

§4.1 The MAC is HMAC-SHA256 computed with the key selected by the header's
`kid` over the exact ASCII bytes:

```
headerSegment + "." + bodySegment
```

That is: the encoded header segment, a single dot, and the encoded body
segment, with no version prefix and no MAC segment. The MAC must be computed
over the encoded segments as transmitted.

§4.2 Because the header is covered, no header field may be altered without
knowledge of the key. In particular, changing `kid`, changing `comp`, adding,
removing, or reordering header members invalidates the cookie.

§4.3 The received tag and the expected tag are compared in constant time over
their full length. Comparison must not return early on the first differing
byte and must not leak the position of a mismatch. A tag with the wrong length
is invalid.

§4.4 Verification uses exactly one key: the key named by `kid` (§5). It must
never try multiple keys, fall back to a default key, or accept a cookie when
key selection fails.

## §5 Key ring and rotation

§5.1 Every key in a reader's ring has a `kid`, key material, and a validity
window. The `kid` namespace is unique within a ring; a ring that repeats a
`kid` is a configuration error on the reader's side.

§5.2 The reader selects the key whose `kid` equals the header `kid` exactly
(byte-for-byte; no prefix, case, or fuzzy matching).

§5.3 An unknown `kid` is a rejection. There is no default key. The reader must
not fall back to the first, newest, oldest, or any other entry.

§5.4 Keys remain usable inside their validity window only. A key whose grace
period has ended must not be accepted even if it is still physically present
in the ring. Services must reject cookies signed with expired keys rather than
letting them linger.

§5.5 Rotation is issuer-driven: new cookies use the newest active key; readers
may hold older keys during their published grace period so that in-flight
cookies keep working. The grace period is a bound on acceptance, not a promise
that old keys are ignored forever.

## §6 Body compression

§6.1 `body` is compressed with raw DEFLATE (`deflate-raw`, RFC 1951), with no
zlib or gzip wrapper.

§6.2 Decompression is bounded. A reader must not materialise more than
64 KiB (65,536 bytes) of decompressed body per cookie. A body that would
decompress beyond the limit is rejected at the limit; the reader must stop and
report the limit error without growing memory in proportion to the expanded
size. Highly compressible input is not exempt.

§6.3 After decompression the byte sequence must be valid UTF-8 that parses as
a JSON object. Invalid UTF-8, trailing bytes, or a top-level non-object value is
a rejection.

§6.4 A body that is empty, truncated, or not a complete deflate stream is a
rejection.

## §7 Claims

§7.1 Recognised envelope claims, with fixed types:

| claim | type | meaning |
|---|---|---|
| `iss` | string | issuer identifier |
| `sub` | string | subject identifier |
| `aud` | string or array of strings | services allowed to read the cookie |
| `exp` | number | expiry, seconds since the Unix epoch |
| `iat` | number | issued-at, seconds since the Unix epoch |
| `nbf` | number | not-before, seconds since the Unix epoch |
| `jti` | string | unique cookie identifier |
| `scopes` | array of strings | granted permission names |

Times are integer seconds. Fractional values are rejected. A cookie without
`exp` is invalid.

§7.2 Defaults are per read. Readers may supply defaults for claims that are
absent, but defaults must be evaluated for each `read` call in isolation.
No value from a previous read may appear in the result of a later read.
Defaults must not override claims that are present in the cookie.

§7.3 Forbidden claim names. The claims object must not contain `__proto__`,
`constructor`, or `prototype` as members at any depth. A cookie containing any
of them is rejected; it is never partially applied and never sanitised in a
way that changes the meaning of the claims.

§7.4 Claim visibility. A cookie is readable only by services named in `aud`
(a string `aud` is treated as a one-element list). A service that is not in
`aud` must be rejected, even if it holds the key. When `aud` is present, the
check happens on every read. Claims in a cookie addressed to several services
are visible to all of them; cookies intended for different trust boundaries
are issued separately, with different `aud`, so that a compromise in one
service cannot replay another service's cookie. Issuers set `aud` on every
cookie that crosses a trust boundary.

§7.5 Expiry and validity window. A reader rejects a cookie when `now` is at or
after `exp`, when `now` is before `nbf`, or when `iat` lies in the future
beyond the permitted clock skew (default 60 seconds). `exp` must be after
`iat` and after `nbf`. Expired cookies are rejected on every read; a reader
must not return cached claims for a cookie whose `exp` has passed.

§7.6 `read` returns a fresh plain object. The returned object must not share
mutable state with the key ring, the defaults configuration, or any object
retained between calls. Mutating a result must not affect a later read.

## §8 Size limits

| limit | value |
|---|---|
| maximum cookie length, ASCII bytes | 4096 |
| maximum header JSON bytes, decoded | 1024 |
| maximum decompressed body bytes | 65536 |

A cookie exceeding any limit is rejected with `E_TOO_LARGE` (or the more
specific limit code in the implementation's error table) before any claims are
returned. Length is measured on the string as received; no normalisation is
performed first.

## §9 Rejection and error reporting

Every failure is reported as a rejection with a stable code, for example:

- `E_FORMAT` — wrong number of segments, bad prefix, non-ASCII input
- `E_ENCODING` — non-canonical or invalid base64url
- `E_HEADER` — header missing, malformed, unknown `comp`
- `E_UNKNOWN_KID` — `kid` not in the ring
- `E_KEY_EXPIRED` — selected key outside its validity window
- `E_MAC` — MAC missing, wrong length, or mismatch
- `E_INFLATE` — body is not a valid deflate stream
- `E_TOO_LARGE` — a size limit in §8 was exceeded
- `E_CLAIMS` — invalid claims JSON, forbidden name, bad types
- `E_EXPIRED` — cookie outside its validity window
- `E_AUDIENCE` — calling service not permitted by `aud`

A reader either returns verified claims or throws. It never returns partially
verified data, and error paths must not expose key material or raw claims.

## §10 Replay resistance

§10.1 Authenticity is necessary but not sufficient. `read` enforces the
validity window (§7.5) and audience (§7.4) on every call.

§10.2 A cookie is a bearer credential. Services that require one-time use
must combine `read` with their own `jti` replay check; `read` itself is
stateless.

§10.3 The same logical cookie must have one and only one serialisation
(§2.3). Readers, logs, caches, and replay caches key on the cookie string, so
any alternative spelling of the same claims is a distinct credential and must
be impossible.

## §11 Conformance

A conforming reader rejects every malformed or modified cookie, accepts only
cookies signed by an active key with the exact `kid` and valid MAC, enforces
all limits before returning claims, applies per-read defaults, and returns
fresh claims objects.
