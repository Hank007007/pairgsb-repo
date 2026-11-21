# cookieveil — design notes

`cookieveil` is the sealed-cookie codec used at the edge. The login service
issues a cookie once; every downstream service opens it locally with a key it
already holds. There is no shared session store and no per-read network call.

The normative wire contract lives in [`docs/cookie-envelope.md`](docs/cookie-envelope.md).
This note explains how the library is put together and what each part is
responsible for. It deliberately stays at the level of behaviour and
invariants: it states what must be true, not how to make it true.

## Why a sealed cookie

- **No session store.** Each service verifies the cookie with a symmetric key
  and reads the claims it needs. A read is pure CPU and never blocks on
  another service.
- **Trust boundary.** The cookie is a bearer credential. Possessing a valid
  cookie is sufficient to act as its subject, so the envelope must be
  unforgeable and unmodifiable, and the reader must decide which services are
  allowed to use it.
- **Rotation.** Keys are rotated on a published schedule. Old keys remain
  readable for a bounded grace period so in-flight cookies keep working, then
  stop being accepted.

## Shape of the library

```
src/index.js        public API: issue(claims, options) / read(cookie, options)
src/envelope.js     four-segment assembly and strict segment decoding
src/mac.js          HMAC-SHA256 tag computation and verification
src/decompress.js   bounded raw-DEFLATE expansion and claims parsing
src/claims.js       defaults, claim types, audience, validity window
src/keys.js         key ring and kid resolution
src/errors.js       CookieError and stable rejection codes
```

The modules form a pipeline. Each stage either produces a value for the next
stage or throws a `CookieError`. There is no partially-successful read: the
caller receives verified claims or an exception.

## Required behaviour, by stage

### Envelope

A cookie is exactly four segments: `v2.<header>.<body>.<mac>`. Each encoded
segment is the canonical unpadded base64url encoding of the bytes it carries.
Parsing is strict and total:

- the segment count is exactly four — neither surplus nor missing segments
  are tolerated;
- every character belongs to the base64url alphabet;
- no padding characters;
- the decoded bytes re-encode to exactly the received string (canonical
  form);
- the decoded header, body, and MAC are non-empty and fit their size limits.

Because the accepted language is canonical, a given (header, body, mac)
triple has **one** cookie string. This is what lets logs, caches, and replay
stores use the string as an identity.

### Authentication

The tag is HMAC-SHA256 over the encoded header segment, a dot, and the
encoded body segment. The version prefix and the MAC segment are not part of
the signed bytes, but the version is fixed by the parser.

Two properties matter:

1. **Coverage** — every transmitted byte that carries meaning is inside the
   signed input. In particular the header (which names the key and the
   compression algorithm) is authenticated, so altering it invalidates the
   cookie.
2. **Comparison** — the received tag is compared to the expected tag in
   constant time and over the full length. Length mismatch is a rejection.
   Nothing about the comparison may depend on where the first difference is.

### Key ring

A key entry is `{ kid, secret, notBefore?, notAfter? }`, with times in
seconds. Resolution is exact-match on `kid`. An unknown `kid` is a hard
rejection. A key outside its validity window is a hard rejection, even if it
is still present in the ring. The ring is data, not policy: a reader may keep
expired entries for auditing or for the issuer, but it must not accept them.

The order of the ring is not meaningful for reads. (The issuer picks a key
explicitly, or the newest one.)

### Decompression

The body is raw DEFLATE (RFC 1951) of a UTF-8 JSON object. Expansion is
bounded at 64 KiB. The reader must not allocate memory in proportion to the
expanded size of an attacker-chosen stream. After expansion, the bytes are
decoded strictly as UTF-8 and parsed as a JSON object with no trailing
content.

### Claims

`read` performs, in order, type validation, audience check, validity-window
check, and defaulting. All of these apply to every read.

- Types are fixed by §7.1. Times are integer seconds; `exp` is mandatory.
- Audience: a cookie may only be read by a service named in `aud`. Holding
  the key does not imply permission.
- Validity: `nbf <= now < exp`, with a bounded clock-skew allowance for
  `iat`/`nbf`. Expiry is evaluated on every call.
- Defaults: applied to claims that are absent, never overriding claims that
  are present, and scoped to a single call. No object, array, or scalar from
  one read is reused in another.
- Forbidden names: `__proto__`, `constructor`, and `prototype` are not valid
  claim names at any depth; a cookie carrying them is rejected.
- The result is a fresh plain object owned by the caller.

## Failure model

All rejections are `CookieError` instances with a stable `code` (see §9 of
the envelope contract). Errors carry no key material and no raw claims.
`read` never returns `undefined`, `null`, or a partial object on failure.

## Performance envelope

A read is O(cookie length) plus bounded decompression, with no I/O. The
64 KiB expansion cap is what keeps a single hostile cookie from becoming a
memory-exhaustion event on a shared gateway process.

## Testing

Tests live under `test/` and use `node:test` only. Key material is generated
with `crypto.randomBytes` at test time; nothing touches the network. The
helpers in `test/helpers.js` provide a fixed clock and a rotated key ring so
that time- and rotation-dependent behaviour is deterministic.
