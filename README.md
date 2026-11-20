# kekseal

Envelope encryption for the internal key-management service.

Each secret record is encrypted with a one-time data encryption key (DEK).
The DEK is wrapped with a key-encryption key (KEK) belonging to a numbered
version in a JSON key ring. Old versions can be retired with `retired_at` and
keep working for a configurable grace period.

The normative description of the record layout, associated data, algorithm
allow-list, and key-ring rules is in [`docs/envelope-format.md`](docs/envelope-format.md).
The rotation contract is in [`docs/rotation.md`](docs/rotation.md).

## Layout

```
cmd/kekctl/              operator command line
internal/keystore/
  record.go              record type, JSON I/O
  aad.go                 associated-data assembly
  seal.go                Seal and Open
  ring.go                key ring parsing and lookup
  rotate.go              KEK rotation
  keystore_test.go       package tests
docs/                    format and rotation specification
testdata/rings/          sample key rings
testdata/records/        pre-sealed sample records
tools/genfixtures/       regenerates testdata/ from fixed test keys
```

Only the Go standard library is used.

## Library

```go
ring, err := keystore.LoadRing("ring.json")
sealer := keystore.NewSealer(ring)

rec, err := sealer.Seal("acme", "doc-0001", plaintext)
plaintext, err := sealer.Open(rec)

res, err := sealer.RotateKEK(records)
```

## Command line

Build the tool with `go build ./cmd/kekctl`.

```
kekctl ring init -out ring.json -versions 2 -grace 24h
kekctl seal  -ring ring.json -tenant acme -id doc-0001 -in secret.txt -out rec.json
kekctl open  -ring ring.json -in rec.json
kekctl rotate -ring ring.json -dir records/
```

`kekctl rotate` reads every `*.json` record in the directory, rotates the
records as one batch, and writes them back.

## Tests

```
go test ./...
```

The fixtures under `testdata/` are sealed with the keys embedded in
`testdata/rings/ring-v1-v2.json`; `ring-v1-only.json` contains just the first
version for exercising older-record scenarios.
