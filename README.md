# rotseal

`rotseal` keeps local application logs in a form that can be shown to have
been written once and never edited: it rotates an NDJSON log into blocks,
seals every block with an HMAC signature and a read-only archive directory,
verifies the resulting chain, and retires old blocks under a retention policy.

```
app ──append──▶ active.ndjson ──rotate──▶ segments/segment-000001.ndjson
                                              ├─ seal ──▶ segment-000001.seal
                                              └─ verify ──▶ chain report
```

## Install

The project uses the Python standard library only. Python 3.9 or newer.

```
python -m pip install -e .
```

or run it from a checkout with `python -m rotseal ...`.

## Quick start

```sh
export ROTSEAL_KEYRING=$PWD/keyring.pickle

python -m rotseal init
python -m rotseal key add --id key-2026-01      # creates the keyring, key is active
python -m rotseal append app.ndjson             # copy application records in
python -m rotseal rotate --max-bytes 1048576    # or --every 1h
python -m rotseal seal                          # sign + freeze the segments
python -m rotseal verify --from 0
```

`rotseal` reads and writes one archive directory, taken from `--archive`, then
`$ROTSEAL_ARCHIVE`, then `./archive`.

## Commands

| command | what it does |
|---|---|
| `init` | creates the archive directory layout |
| `append <file>` | appends NDJSON records to `active.ndjson` |
| `rotate [--max-bytes N] [--every 1h]` | closes the active log into numbered segments |
| `seal [--key ID]` | signs every unsealed segment, writes `.seal` sidecars, makes `segments/` read-only |
| `verify [--from N] [--to M] [--json]` | checks the sealed chain and prints per-block results |
| `prune --keep 30d [--dry-run]` | removes sealed blocks outside the retention window |
| `status` | prints the archive index and the active key |
| `key add --id ID` / `key list` | manage the keyring |

`verify` prints one line per checked block, followed by warnings, notes and
failures, and a final summary; `--json` emits the same report as a document.

## Keys

Keys live in a keyring file referenced by `$ROTSEAL_KEYRING` (or
`--keyring`). Each key gets a stable id, which is written into every sidecar
that key signs. `$ROTSEAL_ACTIVE_KEY` overrides which key signs new blocks.
Rotating keys does not invalidate blocks signed with earlier keys; see
[`docs/sealing.md`](docs/sealing.md).

## Documentation

- [`docs/archive-format.md`](docs/archive-format.md) — block naming, sidecar
  fields, ordering and sequence continuity.
- [`docs/sealing.md`](docs/sealing.md) — what a signature covers, key rotation,
  what verification measures.
- [`docs/retention.md`](docs/retention.md) — expiry, tombstones, verifying an
  archive whose oldest blocks were retired.

## Tests

```sh
python -m pytest -q -p no:cacheprovider
```
