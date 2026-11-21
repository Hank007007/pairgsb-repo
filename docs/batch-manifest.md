# Batch manifest and signing

## 1. Fields

A manifest is a flat JSON object with exactly these members:

| field | type | meaning |
|---|---|---|
| `origin` | string | producer identity, `host` + `#` + `instance` |
| `epoch` | integer ≥ 0 | numbering generation for that origin |
| `firstSeq` | integer ≥ 0 | sequence number of the first record in the batch |
| `count` | integer ≥ 1 | number of records in the batch payload |
| `digest` | string | `sha256:<lower-case hex>` of the payload bytes |
| `keyId` | string | identifier of the batch key used to sign |
| `signature` | string | base64 HMAC-SHA256 over the signing input (§3) |

Rules:

* All six non-signature fields are mandatory. A manifest missing any of them is
  rejected with an error naming the missing field and, where known, the origin.
* `firstSeq` plus `count` defines the sequence range
  `[firstSeq, firstSeq + count - 1]`. The payload must contain exactly `count`
  records, and they must be the records with those sequence numbers.
* `digest` is computed over the payload bytes exactly as transmitted, before
  any receiver-side interpretation. It is **not** a digest of the manifest.
* `keyId` must identify a key that is authorised for this `origin` (§4).
* Unknown extra members are ignored for validation but are included in the
  signing input (§3); producers should not add any.

## 2. Canonical JSON

The manifest is serialised with a deterministic canonical form so that the
sender's bytes and the receiver's reconstruction agree:

1. No insignificant whitespace: separators are `,` and `:`.
2. Object members are sorted by key in ascending order of their UTF-16 code
   units (equivalently, for the ASCII key set used here, lexicographic order).
3. Strings are emitted with `"` and `\` escaped, plus the short escapes
   `\b`, `\f`, `\n`, `\r`, `\t`.
4. Every other character outside the printable ASCII range `U+0020..U+007E` is
   emitted as a lower-case `\uXXXX` escape. Code points above `U+FFFF` are
   emitted as a UTF-16 surrogate pair, each half escaped.
5. Numbers are emitted in JavaScript shortest round-trip form; the manifest
   only uses non-negative integers.
6. `true`, `false`, and `null` are literal.

Canonical JSON exists so that two texts that denote the same value (different
key order, differently escaped non-ASCII, different whitespace) produce the
same signing input. Verification therefore recomputes the signing input from
the manifest **value** using exactly the serialisation rules of this section.
The same manifest value must verify identically regardless of how the sender
encoded it, and repeated verification of one manifest must be stable.

## 3. Signing input

The signing input is the canonical JSON of the manifest **with the
`signature` member removed**. Formally:

```
signingInput = canonicalJson({ k: v for k, v in manifest if k != "signature" })
signature    = base64(HMAC-SHA256(secret, UTF-8(signingInput)))
```

Notes:

* The payload is not part of the signing input; it is covered by `digest`,
  which is.
* The signature is compared in constant time against the recomputed value.
  Ordinary string equality is not acceptable for secret-derived values.
* Signature verification is deterministic: for a fixed manifest value and key
  it must produce the same result on every call and in every process.

## 4. Key binding

Each key is bound to a set of origins. A key may only sign batches whose
`origin` it is authorised for. A collector configured with
`{ "key-a": { "secret": "...", "origins": ["web-01#a"] } }` must reject a
manifest signed with `key-a` that claims `origin: "web-02#b"`, even though the
signature itself is valid. An empty `origins` list means "no origin is
authorised"; a collector that intends unrestricted keys must list them
explicitly.

## 5. Rejection reporting

Every rejection must identify:

* the failing `field` (`origin`, `epoch`, `firstSeq`, `count`, `digest`,
  `keyId`, `signature`, or `manifest`/`payload` for structural failures),
* the `origin` being claimed, when it could be parsed,
* a stable machine-readable `code`.

A receiver must not accept a batch whose `count`, `firstSeq`, or `digest`
disagrees with the payload, with the signature, or with what the receiver has
already stored. In particular, changing any one of those fields in a captured
manifest must cause rejection, and the error must name that field and the
claimed `origin`. Structural, payload-derived and signature failures are all
in scope: a changed field must never be reported as an unrelated field or as a
bare "invalid manifest".

## 6. Transport

```
POST /batches
content-type: application/x-spoolbatch
x-spoolbatch-manifest: base64(manifest JSON text)
<body>: raw payload bytes (NDJSON, one record per line)
```

`GET /status` returns per-origin watermark and gap information in the shape
described in `delivery.md`. The response body of a successful `POST /batches`
is described there as well.
