# sealrelay

`sealrelay` moves named secrets from a control plane to edge nodes. Each node
registers an X25519 public key; the control plane seals a bundle for that node
with an ephemeral ECDH handshake and ships it next to an Ed25519-signed
manifest. The edge agent verifies the manifest, decrypts the bundle, and
materializes the secrets under `secrets/`.

The protocol is specified in [`docs/seal-protocol.md`](docs/seal-protocol.md).
That document is normative: it states what a conforming implementation must do
and intentionally describes no implementation.

## Layout

| Path | Role |
|---|---|
| `cmd/sealrelay` | Small driver: `keygen`, `seal`, `accept` |
| `internal/nodekeys` | X25519 identities and node IDs |
| `internal/derive` | Shared-secret to content-key derivation |
| `internal/seal` | AES-GCM envelope encryption |
| `internal/manifest` | Manifest shape, signing, verification, revision policy |
| `internal/envelope` | Wire representation |
| `internal/bundle` | Plaintext secret set and name validation |
| `internal/control` | Control-plane sealing |
| `internal/agent` | Edge-side acceptance, persistence, state |

## Requirements

- Go 1.24 or newer (standard library only).

## Build and test

```sh
go build ./...
go test ./...
```

## Try it

```sh
# Control plane: signing key + a node identity in ./run/demo
go run ./cmd/sealrelay keygen -out run/demo

# An edge node owns its own node.json
mkdir -p run/demo/edge
cp run/demo/node.json run/demo/edge/node.json
cp run/demo/signing.pub run/demo/edge/signing.pub

# Write run/demo/request.json (see below) and seal it
go run ./cmd/sealrelay seal \
  -signing run/demo/signing.key \
  -request run/demo/request.json \
  -out run/demo/envelope.json

# Edge: accept and write secrets
go run ./cmd/sealrelay accept -node run/demo/edge -in run/demo/envelope.json

# Inspect a manifest signature without touching disk
go run ./cmd/sealrelay verify -pub run/demo/signing.pub -in run/demo/envelope.json
```

`request.json`:

```json
{
  "bundle": "payments",
  "revision": 1,
  "expiresAt": 1900000000,
  "recipient": "<node id from node.json>",
  "nodePublic": "<public hex from node.json>",
  "secrets": { "api-token": "tok-123", "db-password": "hunter2" }
}
```

An accepted envelope leaves `secrets/<name>` files and `state.json` in the node
directory.
