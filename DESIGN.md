# keywaytransit — design notes

This document states the behaviour the service is expected to provide. It is
written as a contract: what callers, operators and auditors may rely on. It
does not prescribe an implementation.

The binding specifications live in `docs/`:

- `docs/blob-format.md` — the on-the-wire blob layout and its authentication.
- `docs/key-lifecycle.md` — versions, rotation, destruction, rewrap, tenancy.
- `docs/context.md` — context serialisation and binding.
- `docs/api.md` — HTTP surface, limits, error reporting, audit.

Where this note and the documents above disagree, the documents above win.

## 1. What the service is

A single long-running process serves many tenants. A tenant asks the service to
encrypt a plaintext under a logical key name, and stores the resulting opaque
blob in its own database. Later the tenant hands the blob back and receives the
plaintext. Key material never leaves the service.

Every operation is scoped to a tenant. Key names are chosen by tenants and are
not globally unique. Two tenants may each own a key called `billing`; those are
different keys with different material, and a blob produced under one must not
be readable under the other.

All encryption is AES-256-GCM.

## 2. The blob

A blob is self-describing: it carries the magic, a blob-format version, the key
version, the key name, the nonce and the ciphertext. It contains no tenant
identifier; tenancy is established by the request, not by the payload.

Properties that must hold for every blob the service emits:

1. **Exact layout.** Field order, widths and byte order are exactly those of
   `docs/blob-format.md` §1.1.
2. **Header authenticated.** Every byte of the header (everything before the
   nonce) is covered by GCM authentication. Any change to any of those bytes
   makes decryption fail; no plaintext is released.
3. **Format version honoured.** A decoder must accept only the blob-format
   versions it understands and reject the rest.
4. **Nonce fresh.** The nonce is drawn fresh from `os.urandom(12)` for each
   encryption. Encrypting the same plaintext twice under the same key version
   yields two different nonces and two different ciphertexts.
5. **Key name preserved.** A key name is carried through encryption and
   decryption without loss. If a key name cannot be represented in the header,
   the operation fails rather than silently using a different name.

## 3. Keys and versions

- A key has one or more versions. Each version has its own 256-bit material.
- Encryption always uses the key's current latest version and writes that
  version into the blob header.
- Decryption selects key material by the version recorded in the blob header.
  A blob whose version is not held by the key cannot be decrypted.
- `rotate` adds a version and leaves the key name and tenant unchanged.
- `min_decrypt_version` is the floor below which blobs are no longer accepted.
  A blob below the floor fails with `key_version_gone`, regardless of whether
  material for that version happens to remain loaded.
- Destroying a version removes that version's material everywhere the process
  can reach it — both the persisted record and anything cached in memory.
  Afterwards no blob of that version decrypts; other versions of the same key
  are unaffected.
- `rewrap` moves a blob to the key's latest version without requiring the
  caller to reproduce the original context. The context binding carried by the
  source blob is preserved; callers cannot substitute a different context
  during rewrap.

## 4. Context binding

A context is an optional set of string pairs. It is used to pin a ciphertext to
a semantic location (tenant, field, record).

- Serialisation is canonical: ordered by the byte order of keys, escaped, and
  unambiguous. Distinct contexts produce distinct serialisations.
- "No context supplied" and "an empty context supplied" are different inputs
  and must produce different bindings. A blob encrypted without a context must
  not decrypt when an empty context is supplied, and vice versa.
- On decryption the supplied context must reproduce exactly the binding the
  blob was created with. Any difference — an added pair, a removed pair, a
  changed value — fails with `context_mismatch` and releases no plaintext.
- Context comparison is a property of the serialised binding, not of the
  insertion order of the caller's mapping.

## 5. HTTP surface

- Requests are JSON, scoped by a tenant identifier that comes from request
  metadata rather than the body.
- `/v1/batch` accepts at most 64 items. A larger request is rejected as a
  whole; no item is processed and no partial results are returned.
- Error responses contain a stable error code. They may additionally name the
  key involved. They never contain exception text, stack traces, plaintext,
  plaintext lengths, or other internal detail.
- Distinct failure causes are distinguishable by code: a destroyed or
  below-floor version, a context mismatch, an authentication failure, an
  unparseable blob, an oversized batch, and a malformed request each have
  their own code.

## 6. Audit

Each operation may be audited. An audit record identifies who did what: tenant,
key name, operation, version, outcome, timestamp. It must not contain the
plaintext, and it must not contain a wholesale copy of the request body (which
carries plaintext on the encrypt path).

## 7. Operational shape

- Key records are persisted under a directory given at startup; the process
  also keeps a working set in memory.
- The persisted and in-memory views must not diverge in ways that change what
  is decryptable: a destroyed version must be undecryptable through either
  path.
- The process survives malformed input; callers receive an error code rather
  than a connection-level failure or an internal trace.
