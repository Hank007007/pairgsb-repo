# beaconproof

beaconproof keeps a tamper-evident local audit log and anchors it to an
external timestamp authority.

- Audit events are appended to a hash chain (`seq`, time, actor, action, body).
- Every so many records, the records in that interval are reduced to a Merkle
  root and submitted to a timestamp authority. The authority returns a signed
  receipt, which is stored under `receipts/`.
- `beaconctl verify` replays the local chain and checks every stored receipt
  against the chain.

The normative behaviour is specified in:

- [`docs/chain-format.md`](docs/chain-format.md) — record fields, hash input,
  timestamps, sequence numbers.
- [`docs/anchoring.md`](docs/anchoring.md) — when to anchor, how failures and
  outages are handled, how intervals are cut.
- [`docs/receipts.md`](docs/receipts.md) — which receipt fields are checked and
  how intervals relate to one another.
- [`docs/design.md`](docs/design.md) — the design statement behind them.

## Layout

```
cmd/beaconctl        command line entry point
internal/chain       record encoding, append, replay verification, lookups
internal/merkle      leaf/node hashing and range roots
internal/anchor      anchoring policy, submission, catch-up
internal/receipt     receipt storage and verification
internal/tsa         HTTP client and an in-process timestamp authority
```

## Build and test

```
go build ./...
go test ./...
```

## Try it

The repository ships an in-process timestamp authority so the whole flow can be
exercised locally. In one terminal:

```
go run ./cmd/beaconctl authority --addr 127.0.0.1:8787
```

In another, using a scratch directory:

```
export BEACONPROOF_HOME=$(mktemp -d)
go run ./cmd/beaconctl append --actor alice --action login --body "session 1"
go run ./cmd/beaconctl append --actor alice --action read  --body "doc 7"
go run ./cmd/beaconctl anchor --n 2
go run ./cmd/beaconctl verify
```

`BEACONPROOF_HOME` selects the directory holding `audit.log`, `receipts/`, and
the authority key pair written by the `authority` command.

## Commands

| command | purpose |
|---|---|
| `append` | append one event to the local log |
| `anchor` | submit the pending interval for a receipt (`--n` records per interval, `--max-age` time trigger) |
| `verify` | replay the chain and check every receipt |
| `authority` | run a local timestamp authority |

`verify` exits non-zero when it reports any finding.
