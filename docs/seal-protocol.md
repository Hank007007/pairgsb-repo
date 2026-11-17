# Seal relay protocol

Status: normative specification. This document defines what a conforming
control plane and edge agent must do. It deliberately says nothing about how
the implementation is organized.

Terms used below:

- **control plane** — the process that seals bundles and signs manifests.
- **edge agent** — the process that receives envelopes and writes secrets.
- **bundle** — a named set of secrets (name → value) belonging to one tenant.
- **recipient** — the edge node an envelope is addressed to, identified by the
  printable ID derived from that node's X25519 public key.
- **envelope** — the wire object delivered to an edge node.

## §1 Envelope

§1.1 Every edge node owns an X25519 key pair. Its node ID is derived from the
public key and is stable for the lifetime of the key pair.

§1.2 An envelope carries: a format version, an ephemeral X25519 public key, a
nonce, the ciphertext, and a signed manifest. The ciphertext is an AEAD
encryption of the bundle contents under the content key defined in §2.

§1.3 The manifest carries `bundle`, `revision`, the sorted list of secret
names, a digest of the plaintext bundle, the `recipient` node ID, an
`expiresAt` absolute Unix timestamp, and a control-plane signature.

§1.4 Ciphertext is authenticated. Any bit flip in the ciphertext, nonce, or
associated data must cause decryption to fail, and the agent must not write
anything to disk.

## §2 Content-key derivation

§2.1 The content key is derived from the X25519 shared secret between the
sender's ephemeral key and the recipient's public key.

§2.2 The derivation input must bind both the `bundle` name and the recipient
node's public key (the same bytes the recipient advertises). Consequently:

- two different bundles sealed for the same node must never yield the same
  content key, even if the shared secret were identical;
- the same bundle sealed for two different nodes must never yield the same
  content key.

§2.3 The control plane and the edge agent must perform the identical
derivation for a given envelope. A mismatch must surface as a decryption
failure, never as silently wrong plaintext.

## §3 Nonce handling

§3.1 Every AEAD encryption must use a nonce that has never been used before
with that content key, and must never be used again with it.

§3.2 Uniqueness must survive process restarts. A restarted control plane must
not resume from a nonce value that an earlier process already used.

§3.3 Re-sealing the same bundle, with the same revision or a new one, must
produce a distinct nonce. The nonce is carried in the clear and is part of the
delivered envelope.

## §4 Manifest signature

§4.1 The control plane signs the manifest with Ed25519. The edge agent holds
the corresponding public key out of band.

§4.2 The signed message must cover every field that influences acceptance:
`bundle`, `revision`, the secret-name list, the plaintext digest, `recipient`,
and `expiresAt`.

§4.3 Changing any covered field after signing must make verification fail.
In particular, retargeting an envelope to another node by editing `recipient`,
or extending its life by editing `expiresAt`, must be rejected as unauthentic.

§4.4 Verification happens before the ciphertext is decrypted and before any
secret is written.

## §5 Revision monotonicity

§5.1 The agent persists the revision of the last bundle it applied.

§5.2 An envelope is admissible only when its `revision` is strictly greater
than the locally persisted revision for that bundle. Equal or lower revisions
are rejected.

§5.3 A rejected envelope must leave the previously applied secrets untouched:
no secret from it is written and no persisted state advances.

§5.4 The revision check is evaluated before decryption and before any write.

## §6 Expiry

§6.1 `expiresAt` is an absolute Unix timestamp in seconds.

§6.2 An envelope whose `expiresAt` is at or before the agent's current time is
rejected. Rejection happens before any secret is written.

§6.3 Expiry is a hard cutoff: there is no grace window and no clock skew
allowance beyond the timestamp comparison itself.

## §7 Applying a bundle

§7.1 Accepted secrets are written to `<agent root>/secrets/<name>`, one file
per name, containing exactly the secret value.

§7.2 Names are validated: empty names, names containing a path separator, and
`.`/`..` are rejected.

§7.3 The set of files under `secrets/` reflects the bundle last applied and
nothing else. After applying a new revision, any name that existed in the
previous revision but is absent from the new one must be removed from disk.

§7.4 The persisted state (bundle name, revision, applied names, applied time,
expiry) advances only after the new secrets have been written successfully.

§7.5 Rejected envelopes (signature, recipient, expiry, revision, decryption,
digest, name-list mismatch) leave `secrets/` and the persisted state exactly
as they were.
