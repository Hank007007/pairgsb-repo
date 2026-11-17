# kekring

Envelope encryption for an object store, plus a small HTTP interface.

Each object gets a fresh random data key (DEK). The DEK is wrapped with a
key-encryption key (KEK) from a versioned keyring, and the wrapped DEK, KEK
version and wrapping algorithm travel in the object header. The body is
encrypted under the DEK. Rotation adds a key version and rewraps headers only;
object bodies are never rewritten. Retired versions stay in the keyring so old
objects keep opening.

The normative behaviour is in [`docs/envelope-spec.md`](docs/envelope-spec.md).

## Layout

| path | contents |
|---|---|
| `internal/keyring` | keyring document, key derivation, version selection, rotation |
| `internal/wrap` | wrapping and unwrapping of data keys |
| `internal/envelope` | object header, seal / open / rewrap |
| `internal/httpapi` | HTTP handlers |
| `cmd/kekringd` | HTTP server |
| `cmd/kekringctl` | keyring file tooling |

## Build and test

```sh
go build ./...
go test ./...
```

## Keyring files

A keyring document is JSON. `kek` values are base64-encoded raw bytes.

```json
{
  "format": 1,
  "keys": [
    {"version": 1, "status": "retired", "kek": "..."},
    {"version": 2, "status": "active",  "kek": "..."}
  ]
}
```

Create and rotate keyrings from a master secret:

```sh
# 32 bytes of master keying material, hex encoded
export MASTER_HEX=$(openssl rand -hex 32)

go run ./cmd/kekringctl init -master-hex "$MASTER_HEX" -version 1 -out keyring.json
go run ./cmd/kekringctl rotate -master-hex "$MASTER_HEX" -keyring keyring.json
go run ./cmd/kekringctl status -keyring keyring.json
```

## Running the server

```sh
go run ./cmd/kekringd -addr 127.0.0.1:8080 -keyring keyring.json
```

```sh
curl -sS -X PUT --data-binary 'hello' localhost:8080/objects/notes/a.txt
curl -sS localhost:8080/objects/notes/a.txt
curl -sS -X POST localhost:8080/objects/notes/a.txt/rewrap
curl -sS localhost:8080/keyring
curl -sS -X POST localhost:8080/keyring/rotate -d '{"kek":"<base64 32 bytes>"}'
```
