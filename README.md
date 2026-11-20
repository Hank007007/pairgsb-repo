# patchsieve

`patchsieve` is the layer of a content management service that handles
`PATCH /documents/{id}`. The client submits an RFC 6902 operation sequence; the
layer loads the current document, applies the sequence, and writes the whole
document back. Authentication happens at the gateway, so the layer only receives
a `Principal{Role, TenantID}` and must enforce path scope, field permissions, and
atomicity itself.

The behavioral contract is in [docs/patch-policy.md](docs/patch-policy.md).

## Layout

| path | contents |
|---|---|
| `internal/pointer/pointer.go` | JSON Pointer parsing and evaluation (RFC 6901) |
| `internal/patch/apply.go` | request application and per-operation logic |
| `internal/patch/operation.go` | operation decoding and JSON number handling |
| `internal/patch/limits.go` | request limits from the policy |
| `internal/patch/test.go` | value comparison for `test` |
| `internal/policy/fields.go` | field-level write authorization per role |
| `cmd/patchdemo` | runs `testdata/scenarios.json` and prints before/after |

## Usage

```sh
go test ./...
go run ./cmd/patchdemo
go run ./cmd/patchdemo -file testdata/scenarios.json
```

`testdata/scenarios.json` contains one entry per behavior described in the
policy. Each entry names the principal, the current document, and the patch; the
demo prints the document before and after, and the error code when the sequence
is rejected.

## Notes

The implementation uses only the Go standard library.
