# Sealing

Sealing turns a closed segment into an archive block: it computes the block's
descriptive fields, signs them, writes the sidecar, and the block directory is
made read-only to emulate write-once storage.

## 1. Keys and the keyring

- The keyring holds one or more HMAC-SHA256 keys. Each key has a stable
  identifier (`key_id`) that is written into every sidecar the key signs.
- The keyring location is configuration, not data: it comes from
  `--keyring` or the `ROTSEAL_KEYRING` environment variable. The keyring file
  itself is a trusted local file, and the loader must treat its contents as
  input that can be malformed or hostile. A loader must not execute code
  contained in the file.
- One key is *active*: new signatures use it. `ROTSEAL_ACTIVE_KEY` may select
  the active key by id; otherwise the keyring's own `active` field applies.
- Keys are never deleted from the keyring while blocks signed with them still
  exist. A key may stop being active at any time.

## 2. What the signature covers

### 2.1 Signed fields

The signature covers, at minimum, all of the following, in a canonical and
unambiguous serialisation:

| field | reason |
|---|---|
| `segment` | binds the sidecar to one block number |
| `seq_start`, `seq_end` | binds the sequence range |
| `count` | binds the number of records |
| `records_digest` | binds the record content |
| `sealed_length` | binds the byte length at sealing time |
| `prev_tip` | binds the block into its position in the chain |
| `key_id` | prevents re-labelling a signature with another key |

A field that is not covered by the signature is not protected: an attacker
who can write the sidecar can change it freely. Therefore every field whose
value `verify` relies on must be inside the signed material, and `verify` must
compare the signed values against what it measures from disk (§5).

### 2.2 Canonical form

The signed material is deterministic: the same block sealed twice with the
same key must produce the same bytes to sign, regardless of JSON key order or
whitespace. The sidecar stores the signature and the algorithm name
(`hmac-sha256`).

## 3. Sidecar fields

Every sidecar contains:

| field | type | notes |
|---|---|---|
| `version` | int | sidecar format version |
| `segment` | int | block number |
| `segment_file` | string | file name of the block |
| `seq_start` | int | first `seq` in the block |
| `seq_end` | int | last `seq` in the block |
| `count` | int | number of records |
| `prev_tip` | string or null | tip of the preceding block |
| `records_digest` | hex string | digest over the block's records |
| `sealed_length` | int | byte length at sealing time |
| `first_ts`, `last_ts` | string | timestamp of the first and last record |
| `key_id` | string | id of the signing key |
| `alg` | string | `hmac-sha256` |
| `sealed_at` | string | sealing time, UTC |
| `signature` | hex string | signature over §2.1 |
| `tip` | hex string | chain tip contributed by this block |

`tip` is derived from the block's number, sequence range and record digest.
A later block's `prev_tip` equals the earlier block's `tip`.

## 4. Verification across key rotation

Verification must succeed for every block in the requested range, whether it
was sealed with the currently active key or with any earlier key that is
still present in the keyring.

- The verifier chooses the key from the sidecar's `key_id`, looks it up in
  the keyring, and verifies the signature with that key.
- The report names, for every checked block, which key verified it.
- A block whose `key_id` is not in the keyring is a verification failure. It
  is not a warning, and it does not cause the block to be skipped.
- The exit status of the command reflects the whole run: any unverified or
  failed block makes the command exit non-zero.

## 5. What verification measures

Verification does not trust the sidecar. For every block it:

1. reads the block file and checks strict UTF-8 decoding;
2. measures the byte length and compares it with `sealed_length` — a mismatch
   is reported as *content beyond the sealed length* and names the block;
3. parses the records, checks that there are exactly `count` of them, that
   `seq_start`/`seq_end` match the file, and that the sequence is continuous;
4. recomputes the record digest and compares it with `records_digest`;
5. recomputes the signature over the signed fields and compares it with
   `signature`, reporting the field name when a signed field disagrees;
6. recomputes the preceding block's tip and compares it with this block's
   `prev_tip`.

Steps 2, 4 and 5 together mean that any edit to a signed field, any appended
or removed record, and any substituted sidecar is detected, and the report
identifies both the block and the field.
