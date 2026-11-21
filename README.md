# subsetwitness-audit

Verifiable selective disclosure for audit logs, implemented in pure Python
with the standard library only.

An operator keeps an append-only ledger. Periodically (one hour by default)
the events of a period are grouped by actor, each actor's events are sealed
into a Merkle bucket, the buckets are sealed into a directory tree, and the
resulting commitment is appended to the main hash chain. Later, an auditor
can ask for one account's activity without receiving the rest of the ledger,
and can verify the disclosure offline against the chain.

## Install

```
python -m pip install -e .
```

or run from a checkout; the package is importable from the repository root.

## Walkthrough

```python
from subsetwitness.chain import append_commit, read_chain, seal, write_chain
from subsetwitness.extract import extract, make_commitment

events = [
    {"seq": 1, "kind": "event", "ts": "2026-03-01T00:05:00",
     "actor": "alice", "action": "login"},
    {"seq": 2, "kind": "event", "ts": "2026-03-01T00:17:00",
     "actor": "bob", "action": "login"},
]

sealed = seal(events)                       # chain links over the events
write_chain("audit.log", sealed)

commitment = make_commitment(
    sealed, "2026-03-01T00:00:00Z",
    "2026-03-01T00:00:00", "2026-03-01T01:00:00", "operator-key",
)
append_commit("audit.log", commitment)

package = extract(
    read_chain("audit.log"), "alice",
    "2026-03-01T00:00:00", "2026-03-01T01:00:00", commitment,
)
```

On the auditor's machine:

```
subsetctl verify package.json --chain audit.log --key operator-key
```

## Command line

| command | purpose |
|---|---|
| `subsetctl seal` | read events, append them to the chain, and append the period commitment |
| `subsetctl extract` | write a disclosure package for one actor and window |
| `subsetctl verify` | run the offline checks on a package |
| `subsetctl chain` | walk the chain and check its links |

Run `subsetctl <command> --help` for the arguments of each.

## Layout

```
subsetwitness/bucket.py    per-actor Merkle buckets
subsetwitness/dir.py       per-period directory tree over buckets
subsetwitness/extract.py   time windows, commitments, package construction
subsetwitness/verify.py    offline package verification
subsetwitness/chain.py     main hash chain
subsetwitness/cli.py       subsetctl entry point
docs/bucket-tree.md        leaf encoding, ordering, empty/single buckets
docs/disclosure.md         package format, fields that must be verified, privacy
docs/verify.md             checks, report format and exit codes
tests/                     test suite
```

## Tests

```
python -m pytest -q -p no:cacheprovider
```

## Status

This is an initial implementation. The documents under `docs/` are
normative; where the code and the documents disagree, the documents
describe what the system is supposed to do.
