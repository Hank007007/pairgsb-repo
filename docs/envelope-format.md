# Envelope format

This document is the normative description of the on-disk envelope produced by
`internal/keystore`. It describes required behaviour, not implementation.
Where this document and the code disagree, this document wins.

## 1. Goals and threat model

`kekseal` protects secret records with envelope encryption:

* every record is encrypted with a **data encryption key (DEK)** that is used
  exactly once;
* the DEK is **wrapped** by a key-encryption key (KEK) belonging to a numbered
  **key version**;
* KEKs live in a JSON **key ring** managed by the internal key service.

Record files and the key ring file are both treated as data that an attacker
may be able to read and modify. In particular the attacker can edit any header
field of a record, move a record between tenants, present a record under a
different id, or supply a doctored key ring to an operator. Authenticity of
header fields is therefore established with AEAD associated data, and
structural inconsistencies are rejected.

The KEKs themselves are assumed to be secret.

## 2. Record

A record is a single JSON object with these fields, all required:

| Field         | Type          | Meaning                                            |
|---------------|---------------|----------------------------------------------------|
| `id`          | string        | Record identifier, unique within a tenant.         |
| `tenant`      | string        | Owning tenant.                                     |
| `key_version` | integer ≥ 1   | Key version whose KEK wraps the DEK.               |
| `alg`         | string        | Payload algorithm. See §4.                         |
| `nonce`       | base64 string | AES-GCM nonce used for the payload.                |
| `wrapped_dek` | base64 string | Version, wrap nonce, and sealed DEK (see below).   |
| `ciphertext`  | base64 string | Sealed payload (ciphertext ‖ tag).                 |

`nonce` is 12 bytes and is drawn fresh from a cryptographically secure random
source for every sealing operation. `ciphertext` is the plaintext followed by a
16-byte GCM tag.

`wrapped_dek` is the concatenation of:

1. a 4-byte big-endian unsigned integer holding the **wrap version** — the KEK
   version the DEK was wrapped under;
2. a 12-byte wrap nonce;
3. the DEK sealed with AES-256-GCM under that KEK, i.e. 32 bytes of DEK plus a
   16-byte tag.

The wrap version is a lookup hint: it selects which KEK unwraps the DEK. It
carries no authority of its own, because the record header — including
`key_version` — is authenticated by the associated data in §3. Tampering with
the header must therefore be caught by authentication, not by trusting the
wrap version.

`id`, `tenant` and `key_version` are *header* fields: they are not secret, but
they are **authenticated**. A record read from disk is untrusted input until it
has been authenticated.

## 3. Associated data

The payload is sealed with AES-GCM using associated data (AAD) that binds the
record to its header. The AAD is a deterministic, unambiguous encoding of at
least the following, in this order:

1. `id`
2. `tenant`
3. `key_version`

"Unambiguous" means that two distinct header tuples must never encode to the
same byte string; field boundaries must be encoded, not implied by plain
concatenation. A length-prefixed or otherwise self-delimiting encoding
satisfies this requirement.

Consequences that follow from the requirement, and that the implementation
must exhibit:

* Opening a record with a different `tenant`, a different `id`, or a different
  `key_version` than the one it was sealed under **must fail authentication**
  with `ErrAuthentication`; no plaintext, partial or otherwise, is returned.
* The same AAD derivation is used when sealing, when opening, and when
  re-wrapping during rotation. A record sealed by one path must be openable by
  every other path, and vice versa.
* Changing `tenant`, `id` or `key_version` in a record and then opening it is a
  tampering attempt, not a rename operation.

The DEK wrap is separately authenticated. Its associated data binds the record
identity, so a wrapped DEK cannot be grafted onto a different record.

## 4. Algorithms

`alg` is a **closed allow-list**. The only value defined by this version of the
format is:

| `alg`       | Meaning                                   |
|-------------|-------------------------------------------|
| `"aes-gcm"` | AES-256-GCM payload, AES-256-GCM DEK wrap |

Any other value — including `"plain"`, the empty string, mixed-case variants,
or unknown future names — **must be rejected** with `ErrUnsupportedAlgorithm`
before any key material is touched. There is no passthrough mode: a record
whose `alg` is not exactly `"aes-gcm"` never yields plaintext, regardless of
what its `ciphertext` contains.

## 5. Open

Opening a record proceeds as follows:

1. Validate the record shape: non-empty `id`, non-empty `tenant`,
   `key_version` ≥ 1. Violations produce `ErrInvalidRecord`.
2. Check `alg` against the allow-list (§4).
3. Read the wrap version from `wrapped_dek` and look up that KEK (§6). Unknown
   versions produce `ErrUnknownVersion`; versions whose grace period has
   expired produce `ErrVersionRetired`.
4. Unwrap the DEK. Failure is `ErrAuthentication`.
5. Decrypt the payload with the AAD from §3. Failure is `ErrAuthentication`.

Every failure returns an error; `Open` never returns data alongside an error.

## 6. Key ring

The key ring is a JSON document:

```json
{
  "grace_period": "24h",
  "versions": [
    {
      "version": 2,
      "kek": "<base64, 32 bytes>",
      "created_at": "2025-03-01T00:00:00Z",
      "retired_at": "2025-06-01T00:00:00Z"
    }
  ]
}
```

Rules:

* `versions` must be non-empty. Each `version` must be a positive integer,
  each `kek` must decode to exactly 32 bytes, and timestamps must be RFC 3339.
* Versions must be **unique**. A key ring that names the same version more
  than once is malformed and must be rejected wholesale with an error; it is
  never acceptable to silently let one entry replace another, because that
  would let a doctored ring shadow a legitimate KEK.
* `grace_period` is optional and defaults to zero. It is a Go duration string
  (for example `"24h"`) and must not be negative.
* A version is **active** while `retired_at` is absent. The active version of a
  ring is the highest-numbered active version; sealing always uses it.
* A version with `retired_at` set is **in its grace period** while
  `now < retired_at + grace_period`. During that window it can still open
  records sealed under it.
* Once `retired_at + grace_period <= now`, the version is **expired**. Records
  still referring to it cannot be opened; the attempt fails with
  `ErrVersionRetired`. Expired KEKs must not be used to unwrap anything.
* A ring with no active version cannot seal; `ActiveKEK` reports
  `ErrNoActiveVersion`.

Version numbers are monotonically increasing. A new version is introduced by
adding an entry with a higher number; retiring an older version is done by
setting its `retired_at`.

## 7. Errors

The package exposes sentinel errors for programmatic handling. Callers use
`errors.Is`; wrapped errors carry additional context.

| Sentinel                  | Meaning                                                     |
|---------------------------|-------------------------------------------------------------|
| `ErrAuthentication`       | AEAD verification failed (header tampering, corruption, wrong key) |
| `ErrUnsupportedAlgorithm` | `alg` not in the allow-list                                 |
| `ErrUnknownVersion`       | `key_version` not present in the ring                       |
| `ErrVersionRetired`       | key version past its grace period                           |
| `ErrDuplicateVersion`     | key ring names the same version more than once              |
| `ErrNoActiveVersion`      | ring has no version able to seal                            |
| `ErrEmptyRing`            | ring contains no versions                                   |
| `ErrInvalidRecord`        | record fails structural validation                          |
| `ErrInvalidRing`          | key ring file is malformed                                  |
| `ErrRotationFailed`       | a rotation could not be completed as a whole                |
