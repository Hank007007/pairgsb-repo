# logline-audit-encode

Encodes audit events as one-line records for a SIEM, in `key=value` text or
JSON. The wire format is specified in [`docs/log-encoding.md`](docs/log-encoding.md);
the package design is described in [`docs/design.md`](docs/design.md).

## Usage

```go
enc, err := encode.NewEncoder(encode.FormatKV)
if err != nil {
    return err
}
record, err := enc.Encode(event)
if err != nil {
    return err
}
// record has no trailing newline; append exactly one '\n' per event.
```

Both formats are also reachable as `encode.FormatJSON`.

## Tools

Encode a sample event:

```sh
go run ./cmd/auditdemo -format kv
go run ./cmd/auditdemo -format json
```

`-hostile` swaps in an event whose user agent contains a CRLF sequence;
`-terminator=false` omits the trailing newline.

Count events as the receiver sees them:

```sh
go run ./cmd/auditdemo -hostile | go run ./cmd/siemcheck
```

Add `-keys` to `siemcheck` to print the field names parsed from each record.

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

Regenerate the baseline goldens after an intentional ordinary-path change:

```sh
go test ./internal/encode -run TestGoldenBaseline -update
```

## Error values

`fields.ErrInvalidKey`, `fields.ErrReservedField`, and
`fields.ErrDuplicateField` report event-rejection conditions described in
the specification; `encode.ErrUnknownFormat` reports an unsupported format.
