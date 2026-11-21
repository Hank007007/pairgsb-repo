# Key handling specification

This document is normative for key material lifecycle and the rules a verifier
applies to keys.

## §K1 Key files

§K1.1 Signing keys live in `keys/`. Each key has a key identifier, a private
key file `keys/<key-id>.key`, and a public key file `keys/<key-id>.pub`.

§K1.2 The key identifier is an operator-visible stable string. It MUST be
unique within a store and MUST appear verbatim in the `key_id` field of every
checkpoint signed with that key (`docs/chain-format.md` §7).

§K1.3 The private key file contains the key in PKCS#8 PEM form with type
`PRIVATE KEY`. It MUST have mode `0600`. The public key file contains the key
in PKIX PEM form with type `PUBLIC KEY` and MUST have mode `0644`.

§K1.4 The private key is never transmitted by the library and is never written
outside `keys/`. Nothing that logs or reports on a store may print private key
bytes.

## §K2 Signing

§K2.1 Only checkpoint signing uses a private key. Records and Merkle nodes are
unkeyed hashes.

§K2.2 The signature covers exactly the message in `docs/chain-format.md` §7.4.
A field that is not part of that message is not protected by the signature and
MUST NOT be relied on for integrity.

§K2.3 Signing MUST use Ed25519 as provided by the Go standard library. The
library MUST NOT implement its own signature scheme or reuse a key for any other
purpose.

## §K3 Verification of keys

§K3.1 A checkpoint is valid only if its `key_id` resolves to a public key file
present in the store and the signature verifies under that key.

§K3.2 An unknown `key_id` is a failure (`unknown-key`), never a warning.

§K3.3 Removing a public key file correspondingly makes every checkpoint signed
by it unverifiable. Key files for historical keys MUST be retained for as long
as the checkpoints they signed are retained.

## §K4 Rotation

§K4.1 Rotation adds a new key; it does not delete or rewrite an old key.

§K4.2 The procedure is:

1. Generate a new key pair under a fresh key identifier (`witnessctl keygen`).
2. Seal the current segment before switching, so that no segment mixes keys.
   Each segment's checkpoint names exactly one `key_id`.
3. Subsequent segments are signed with the new key.
4. Retain all public keys, and retain private keys for as long as their
   checkpoints may need to be re-signed or re-anchored.

§K4.3 Because `key_id` is inside the signed message, an attacker cannot relabel
a checkpoint to a different key without invalidating the signature.

§K4.4 Rotation MUST NOT be performed by editing a checkpoint, its `key_id`, or
its signature in place. A re-signed checkpoint is a new checkpoint with a new
checkpoint hash, and any head anchor that referenced the old hash must be
updated as part of the same coordinated operation.

## §K5 Compromise

§K5.1 If a private key is suspected compromised, an operator MUST publish the
latest known-good checkpoint hash through an external channel, then rotate.

§K5.2 Checkpoints signed by a compromised key cannot be distinguished from
forgeries by verification alone; only external anchoring of hashes taken before
the compromise provides evidence.

§K5.3 Revocation is recorded by removing trust in the key in the external
anchor, not by deleting files from the store; deleting files destroys the
evidence of what was signed.
