# witnesschain

`witnesschain` is a tamper-evident audit log library for control-plane services.
Every event is written as a `Record`; records are linked into a hash chain and
grouped into segments. Each segment ends with a Merkle tree whose root, record
count, and predecessor checkpoint are signed with a local Ed25519 key. The
signed result is a *checkpoint*, stored next to the segment files and listed in
`segments.idx`.

The companion command line tool, `witnessctl`, creates log stores, appends
events, and verifies a store from the beginning, reporting the first position
that does not match.

## Layout

```
cmd/witnessctl/       command line entry point
internal/chain/       records, hash chain, segment sealing, verification, signing
internal/merkle/      Merkle tree, inclusion proofs, proof verification
docs/                 format, verification, and key handling specifications
```

## Build and test

```
go build ./...
go test ./...
```

## Usage

```
witnessctl init   <dir>                     create a store and generate a signing key
witnessctl append <dir> -actor A -action deploy -payload '{"id":7}' -outcome ok
witnessctl verify <dir>                     recheck the whole log
witnessctl keygen <dir>                     add a fresh signing key (rotation)
```

`witnessctl verify` exits non-zero and prints the segment and sequence number of
the first inconsistency it finds.

## Store on disk

```
<dir>/
  keys/<key-id>.key        Ed25519 private key material (0600)
  keys/<key-id>.pub        Ed25519 public key (0644)
  segments/000001.log      JSON Lines, one record per line
  segments/000001.ckpt     signed checkpoint for the segment
  segments.idx             JSON Lines index of segment metadata
```

The normative description of the on-disk format lives in
[`docs/chain-format.md`](docs/chain-format.md). Verification rules are in
[`docs/verification.md`](docs/verification.md), key lifecycle in
[`docs/key-handling.md`](docs/key-handling.md), and the design rationale in
[`docs/design.md`](docs/design.md).
