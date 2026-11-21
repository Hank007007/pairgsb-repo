# maskseal-journal

`maskseal-journal` is a small Go library and command line tool for writing
tamper-evident audit journals whose sensitive fields are redacted **before**
they reach disk.

A caller hands the library an audit record (actor, action, structured fields).
A redaction policy decides which fields must be hidden. A hidden field is not
deleted: its value is replaced by a salted commitment, and the salt is kept in
a separate keystore. The record — including the commitment metadata — is
appended to a hash-chained journal. Someone holding the salt can later prove
that the field really had a particular value; someone reading only the journal
sees a commitment.

An operational path, `maskseal unredact`, retrieves a field's salt after a
second person approves the request, decrypts the sealed value, and records the
unredaction itself as a new audit entry.

## Requirements

- Go 1.22 or newer
- Standard library only

## Build

```sh
go build ./...
go test ./...
go build -o bin/maskseal ./cmd/maskseal
```

## Command line

```sh
# Append a demonstration record.
bin/maskseal append \
  -journal journal-data \
  -salt-dir salt-data \
  -policy examples/policy.json

# Verify the journal. Exits non-zero when the journal does not verify.
bin/maskseal verify -journal journal-data

# Retrieve one field after a second approver signs off.
bin/maskseal unredact \
  -journal journal-data \
  -salt-dir salt-data \
  -seq 1 \
  -field user.password \
  -requester alice \
  -approver bob
```

The demonstration policy in `examples/policy.json` redacts a password, every
SSN in an array, and any field named `password` regardless of its container.

## Package layout

| Path | Purpose |
| --- | --- |
| `internal/record` | record model, canonical encoding, hashing, commitments and sealing |
| `internal/policy` | policy parsing and path matching |
| `internal/keystore` | salt generation and storage |
| `internal/journal` | append-only journal, index, and verification |
| `internal/unredact` | approval tokens and the unredaction workflow |
| `cmd/maskseal` | command line entry point |

## Documentation

- [`docs/journal-format.md`](docs/journal-format.md) — record model, canonical
  encoding, commitments, hashing and chain construction.
- [`docs/redaction-policy.md`](docs/redaction-policy.md) — policy file format,
  path grammar, case handling, arrays, and the default action.
- [`docs/unredact.md`](docs/unredact.md) — approval rules, token binding, salt
  retrieval order, audit trail and rollback behaviour.
- [`docs/design.md`](docs/design.md) — the design contract and the invariants
  the implementation is expected to uphold.

## Journal data

The journal is a directory containing:

- `journal.log` — one canonical encoded record per line;
- `journal.idx` — a line-oriented index of record offsets.

Salts live in a separate directory, one file per salt. The two directories
should be protected independently.
