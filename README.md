# findingsledger

findingsledger is the vulnerability-scan ledger used by our repositories. Each
repository's scanner exports its results to a JSON file; `scanctl` feeds those
exports into the ledger, which recognises the same problem across scans by
fingerprint, keeps its lifecycle state, and turns "ignore this" into a waiver
record that carries a reason, an approval and an expiry date.

The ledger is append-only. Every record has a `seq` and the hash of the record
before it, and every N records a signed checkpoint is written alongside the
ledger. `scanctl verify` recomputes the whole ledger from scratch and checks the
checkpoints.

## Build and test

```
go build ./...
go test ./...
go build -o scanctl ./cmd/scanctl
```

Set `LEDGER_KEY` to the shared signing key before running `scanctl`:

```
export LEDGER_KEY="$(cat /run/secrets/ledger-key)"
```

## Commands

### `scanctl ingest --ledger <file> --scan <export>`

Reads a scanner export and appends a record for every finding that is new to the
ledger. A finding whose recorded state is `fixed` and that shows up again is
recorded as `reopened`.

### `scanctl diff --ledger <file> --scan <export> [--baseline <file>]`

Compares the current scan with the ledger and with an optional baseline (the
scan a branch is allowed to contain). It prints the findings that are new
relative to the baseline, then the findings that the baseline contained but this
scan did not. The command exits with status 1 when the diff contains at least
one new high or critical finding, so a CI job can use it directly as a gate.
`--json` emits the same result as JSON for snapshot tests.

The same inputs must always produce the same output: entries are reported in a
stable order, because CI compares the JSON against a checked-in snapshot.

### `scanctl waiver --ledger <file> --fingerprint <fp> --reason <text> --submitted-by <user> --approver <user> [--expires-in <duration>]`

Records a waiver for a fingerprint. The waiver is signed with the ledger key and
appended to the ledger as a `waiver` record. Waivers default to 30 days.

### `scanctl verify --ledger <file>`

Recomputes every record hash from the record contents, walks the hash chain from
the start, and checks every stored checkpoint against the ledger and its
signature. On failure it reports the first sequence number that does not hold
up, and exits non-zero.

## Layout

| Path | Contents |
|---|---|
| `cmd/scanctl/main.go` | command-line entry point |
| `internal/ledger/record.go` | record and checkpoint format, ledger file I/O, hashing |
| `internal/ledger/fingerprint.go` | fingerprint algorithm |
| `internal/ledger/state.go` | lifecycle state machine, per-finding state |
| `internal/ledger/waiver.go` | waiver structure, signing, approval |
| `internal/ledger/diff.go` | scan/baseline comparison, resolved records |
| `internal/ledger/verify.go` | ledger and checkpoint verification |
| `internal/source/path.go` | path normalization |
| `docs/` | normative specifications |

## Normative specifications

- [`docs/ledger-format.md`](docs/ledger-format.md) — record format, hash chain,
  checkpoints.
- [`docs/fingerprint.md`](docs/fingerprint.md) — how a finding's identity is
  computed.
- [`docs/lifecycle.md`](docs/lifecycle.md) — the status state machine.
- [`docs/waivers.md`](docs/waivers.md) — waiver rules.
- [`docs/diff.md`](docs/diff.md) — how `diff` decides what to report.
