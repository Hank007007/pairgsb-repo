# kekring envelope encryption — design specification

Status: normative. This document states required behaviour only; it does not
describe an implementation.

## 1. Scope

`kekring` provides envelope encryption for an object store. Every object is
encrypted with its own data key (DEK). The DEK is encrypted ("wrapped") with a
key-encryption key (KEK) taken from a versioned keyring. The wrapped DEK and
the parameters needed to open it travel in the object header; the object body
is encrypted directly with the DEK.

Two layers are used:

* body layer — plaintext object bytes under the DEK;
* wrapping layer — the DEK under the KEK.

Both layers use AES-256-GCM.

## 2. Object header

An object header carries at least:

| field | meaning |
|---|---|
| `kek_version` | version number of the KEK that wrapped this DEK |
| `algorithm` | name of the wrapping algorithm, e.g. `AES-256-GCM` |
| `dek_nonce` | 96-bit nonce used at the wrapping layer |
| `sealed_dek` | wrapped DEK (ciphertext followed by the 128-bit tag) |

The body is stored as `body_nonce || body_ciphertext`.

## 3. Wrapping layer

§3.1 The KEK MUST be exactly 32 bytes and AES-256-GCM MUST be used.

§3.2 When wrapping a DEK, the additional authenticated data (AAD) MUST bind all
of the following, unambiguously:

* the object id the sealed DEK belongs to;
* the KEK version recorded in the header;
* the algorithm name recorded in the header.

§3.3 Unwrapping MUST supply exactly the same AAD. A sealed DEK that was
produced for a different object id, a different KEK version or a different
algorithm MUST fail authentication. In particular, transplanting the entire
header of object A onto object B MUST NOT yield a decryptable object.

§3.4 A DEK MUST be 32 bytes. A sealed DEK MUST be exactly
`32 + 16 = 48` bytes, and `dek_nonce` MUST be exactly 12 bytes. Headers that do
not satisfy these lengths are malformed.

## 4. Nonces

§4.1 Every AES-GCM nonce MUST be 96 bits (12 bytes).

§4.2 A nonce MUST be generated from a cryptographically secure random source
and MUST NOT be derived deterministically from any object attribute.

§4.3 Wrapping the same DEK twice MUST produce two different nonces. This
applies to a re-wrap of an object that already has a sealed DEK: the same
object id sealed twice MUST NOT reuse a nonce.

§4.4 Reusing a nonce with the same key and different plaintext is a
catastrophic failure for GCM and is forbidden.

## 5. Key derivation

§5.1 KEKs are derived from a master secret with HKDF-SHA256.

§5.2 The derivation MUST be version-specific: the `salt` and the `info`
arguments are both functions of the key version, and two distinct versions MUST
derive two distinct 32-byte KEKs.

§5.3 The master secret MUST be at least 32 bytes.

§5.4 The derived KEK MUST be exactly 32 bytes.

## 6. Keyring document

§6.1 A keyring document is a JSON object with a `keys` array. Each entry has
`version` (positive integer), `status` (`active` or `retired`) and `kek`
(base64-encoded bytes).

§6.2 Versions MUST be unique; a document with a duplicate version MUST be
rejected.

§6.3 A document with no keys MUST be rejected.

§6.4 A document with an unknown `status` value MUST be rejected.

§6.5 Every KEK in a document MUST be exactly 32 bytes. `Load` MUST reject a
document containing a KEK of any other length, including 16 bytes, at load
time — the error MUST NOT be deferred until the key is first used.

§6.6 The order of entries on disk carries no authority: entries are ordered by
version after loading.

## 7. Key selection

§7.1 `SelectForWrite` MUST return an `active` key. If no `active` key exists
it MUST return an error. It MUST NOT return a `retired` key.

§7.2 `SelectForRead` MUST accept any known version, whether `active` or
`retired`.

§7.3 A `retired` key MUST NOT be used to wrap a new DEK. It remains usable for
unwrapping existing DEKs, forever.

§7.4 Where several `active` keys exist, the highest version is the write key.

## 8. Rotation

§8.1 Rotation adds a new key version and moves the previous `active` version to
`retired`. Retired versions remain in the keyring and remain readable.

§8.2 Rewrapping an object replaces only the header's `kek_version`,
`dek_nonce` and `sealed_dek`. `body_nonce` and the body ciphertext MUST be
byte-for-byte identical before and after the rewrap.

§8.3 Rotation and rewrap MUST be possible while the system is online; no object
body needs to be rewritten.

§8.4 After rotation, newly written objects MUST be wrapped under the new
`active` version, and objects written before rotation MUST still open.

## 9. Failure behaviour

§9.1 Authentication failures at either layer MUST be reported as an error and
MUST NOT return partial plaintext.

§9.2 An unknown `kek_version` in a header MUST be reported as an error rather
than being mapped onto another key.

§9.3 Malformed input (short blob, bad lengths, empty algorithm) MUST be
rejected before any decryption is attempted.

## 10. HTTP interface

| method | path | behaviour |
|---|---|---|
| `GET` | `/healthz` | liveness |
| `PUT` | `/objects/{id}` | seal request body, store it, report the header's `kek_version` |
| `GET` | `/objects/{id}` | open the stored object, return plaintext |
| `POST` | `/objects/{id}/rewrap` | rewrap under the write key, keep the body |
| `GET` | `/keyring` | list versions and statuses |
| `POST` | `/keyring/rotate` | add a new active version and retire the old one |

The HTTP layer is thin: it delegates sealing, opening and rewrapping to the
library and performs no cryptography of its own.
