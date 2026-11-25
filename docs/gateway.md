# protolens gateway

This document describes the request-handling rules of the gateway layer: how a
request body is admitted, how it is authenticated, how the schema is selected,
and how failures are reported. It states required behaviour only.

## 1. Pipeline

A request moves through the gateway in a fixed order:

1. **Admission** — the body is received and its size checked (§3.1).
2. **Version selection** — the schema type and version named by the request are
   resolved once, at the moment processing starts (§3.5).
3. **Decode** — the raw body is decoded with the resolved schema.
4. **Verification** — the HMAC over the raw body is checked (§3.3).
5. **Dispatch** — the decoded object is handed to routing.

Each stage has exactly one source of truth for its input: admission and
verification use the **raw bytes as received**; decode and dispatch use the
**structured object**. The pipeline must not substitute one for the other.

## 2. Request envelope

A request carries, alongside its body:

- `type` — the registered message type name.
- `version` — the schema version the sender encoded against. A request that
  omits it is bound to the version current at admission.
- `signature` — the HMAC over the body (§3.3).
- transport headers, which are metadata and are never part of the signed bytes.

## 3. Rules

### 3.1 Body size limit

- A single request body must not exceed **4 MiB** (4 * 1024 * 1024 bytes).
- The limit is checked against the **actual length of the received body**. A
  `content-length` header, if present, is advisory and must not be trusted as
  the length of the body.
- The check happens **before** any field is read: before the first varint is
  consumed, before the first tag is parsed. An oversized body is rejected
  without the decoder touching it and without buffering additional copies of
  it.

### 3.2 Transport headers

Transport headers describe how a request travelled (hop counts, trace ids,
edge node identity, framing markers and so on). They are not part of the
message and are not part of the signed payload. A gateway may rewrite, add or
drop them without affecting authentication.

### 3.3 Request signature

The signature is an HMAC over the **raw request body exactly as received**.

- Verification must operate on the received bytes. Re-encoding a decoded
  object and signing the result is not equivalent: re-encoding can change
  unknown-field bytes, field order, and the packed/unpacked form, all of which
  are covered by the sender's signature.
- The digest and the expected signature are compared in constant time.
- A signature whose length differs from the expected digest is a verification
  failure, reported as such; it must not raise an unhandled exception. No
  comparison may throw.
- A request that fails verification is rejected before dispatch; the decoded
  object must not reach routing.

### 3.4 Failures and logging

- Decode failures and verification failures are distinct outcomes and must be
  reported with distinct status and a stable machine-readable error code.
- Error responses carry the error code and a short message only. They must not
  echo request bytes, and logs must not record the body or any fragment of it
  in hex or base64.

### 3.5 Schema cache

- Resolved schemas are cached under a two-level key of **(type, version)**.
  A cache entry for `(T, v1)` is never returned for `(T, v2)`.
- The version is fixed for the lifetime of a request: a request resolves its
  schema once at admission (§1, step 2) and every subsequent lookup for that
  request uses the same version, even if the registry is reloaded mid-flight.
- Cache entries may be evicted at any time; eviction must never change
  observable behaviour, only cost.

## 4. Concurrency

The gateway serves requests concurrently. A schema reload taking effect for
*new* requests must not change the schema observed by requests already in
flight. See `schema.md` §2.2.

## 5. Startup and shutdown

The daemon reads its listening configuration from its start-up arguments and
reports success or failure through its exit code. These are part of the
operational contract and are not affected by request-handling rules.
