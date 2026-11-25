# protolens design notes

This note describes the shape of the system and the invariants each layer
upholds. It is descriptive: it explains what the pieces are for and what must
be true of them. The binding rules live in `wire.md`, `gateway.md` and
`schema.md`; this document does not restate them in full and does not dictate
how any of them is implemented.

## 1. Problem

Collectors at the edge encode requests as protobuf and sign them. A request
arrives at the gateway as opaque bytes. The gateway has to answer three
questions, in order:

1. **Is this a well-formed message under the schema the sender used?**
2. **Did the sender really produce exactly these bytes?**
3. **What does the request mean, so it can be routed?**

The first question is about bytes. The second is about bytes. Only the third is
about the decoded object. Keeping those domains apart is the central design
constraint; every rule that follows is a consequence of it.

## 2. Layers

```
          bytes                                    object
 ┌──────────────────────┐   ┌──────────────┐   ┌──────────────┐   ┌────────┐
 │ admission (gateway)  │──▶│ decode (wire)│──▶│ verify (gw)  │──▶│ route  │
 │ size, version pinning│   │ schema-bound │   │ raw bytes    │   │ object │
 └──────────────────────┘   └──────────────┘   └──────────────┘   └────────┘
```

### 2.1 Wire layer (`lib/wire/`)

Owns the byte grammar: varints, tags, lengths, scalar payloads, nesting. It has
no knowledge of HMACs, HTTP, or which request is being served. It is driven
entirely by a *resolved schema* object supplied by the caller.

Its contract is narrow: given bytes and a resolved schema, either return a
complete object or throw a wire error. Partial results are never returned. The
wire layer is also the only place that understands the packed and unpacked
forms of repeated scalars, and the only place that knows how to keep unknown
fields intact.

### 2.2 Schema layer (`lib/schema/`)

Owns the catalogue of message types and their published versions, and turns a
`(type, version)` pair into the immutable object the wire layer needs.

Two properties matter:

- **Publication is monotonic.** A version is immutable once published; a
  change is published as a new version. Nothing revisits an old version.
- **Resolution is snapshot-like.** Once a request has picked a version, the
  object it works with does not change, no matter what the registry does
  afterwards. A reload changes what *new* requests will pick, not what an
  in-flight request already has.

The resolved schema is a small facade: a field list, a field-number lookup and
a way to reach nested message types. Everything in it is derived from exactly
one `(type, version)`.

### 2.3 Gateway layer (`lib/gateway/`)

Owns policy: the size ceiling, the signature scheme, the ordering of checks,
status/code mapping for failures, and dispatch to routing.

The gateway is where the "bytes versus object" distinction is easiest to lose.
Admission and verification operate on the received bytes; decode and dispatch
operate on the object. A request carries a version; the gateway pins it at
admission and never re-reads the registry's notion of "current" afterwards.

## 3. End-to-end flow

1. **Admit.** Measure the body as received. Reject anything over the ceiling
   before the decoder sees a byte.
2. **Pin the version.** Resolve `(type, version)` once. If the request did not
   name a version, take the current one now and treat it as named for the rest
   of the request. Resolution failures are their own class of error.
3. **Decode.** Run the wire decoder with the pinned schema. Unknown fields are
   carried through, not discarded, because step 4 authenticates the original
   bytes and any later use of the object may re-encode it.
4. **Verify.** Compute the MAC over the received body and compare in constant
   time. The comparison is a boolean question; it never raises, including when
   the supplied signature has the wrong shape.
5. **Dispatch.** Hand the decoded object to routing. Nothing about transport
   headers is part of the message or the signature.

Failures at steps 2, 3 and 4 are different outcomes with different status and
code. They are not collapsed into one response, and none of them echoes request
bytes into a response or a log.

## 4. Data ownership

| datum                    | authoritative source | derived / disposable |
|--------------------------|----------------------|----------------------|
| request body             | the wire             | — |
| signature                | the wire             | — |
| transport headers        | transport            | — |
| decoded message          | the body + schema    | yes |
| re-encoded body          | never authoritative  | yes |
| resolved schema          | registry snapshot    | cacheable |

A decoder that wants to know what was signed must look at the body. A router
that wants to know what was asked must look at the message. Neither may
substitute for the other.

## 5. Caching

Caches sit in front of the schema layer. The cache key has to be the same pair
that resolution is defined on, `(type, version)`, because the pair is the
identity of a resolved schema. Caches are permitted to be absent, cold or
evicted at any moment; they are not permitted to be wrong. No cache may outlive
the version it describes, and no cache may conflate two versions of a type.

## 6. Errors

One error type crosses layer boundaries. It carries a stable, machine-readable
code plus a human-readable message. Callers branch on the code; operators read
the message. Codes fall into three families — wire, schema, signature — plus
admission failures. Each family maps to its own response status at the edge.

## 7. Concurrency

Requests are served concurrently and schema reloads can land at any time. The
design has one rule for this: **a request's schema is fixed at admission**.
Everything a request observes downstream is a function of that snapshot. A
reload is a swap of what future requests will see, never a mutation of what
existing requests already hold.

## 8. Out of scope

- Compression, streaming, and chunked transfer framing.
- Groups (wire types 3 and 4) and extensions.
- Proto2 default values, required-field semantics, and text/JSON encodings.
- Transport-level retries, load shedding, and routing policy itself.
