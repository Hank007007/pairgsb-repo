# rotgraph

Third-party credential rotation orchestrator for the platform group.

A *batch* names a set of credentials ("keys") that must be rotated together. For
each key the orchestrator mints a replacement at the vendor, pushes the new
material to every internal consumer over webhook, waits for receipts and for the
dual-live window to elapse, and only then retires the old key. A step that fails
unwinds the batch; a process that dies is picked up from the journal next start.

## Layout

```
rotgraph/
  plan.py          read a batch description, derive the execution order
  journal.py       append-only record of intended external actions
  notify.py        webhook push + receipt collection
  probe.py         ask a vendor whether a key works right now
  runner.py        execute / advance / rollback one batch
  providers/       one adapter per vendor (signer, objstore, paygate, smsgw)
  http/admin.py    admin surface used by rotgraphd
rotgraphd.py       daemon entry point
docs/              the behavioural specification
testdata/          sample plans and recorded vendor responses
tests/             pytest suite
DESIGN.md          how the pieces fit together
```

## Running

```console
python -m pip install -r requirements.txt
python -m pytest -q -p no:cacheprovider
python rotgraphd.py --host 127.0.0.1 --port 8080 --journal-dir logs
```

`rotgraphd` exposes:

- `GET /healthz` — liveness
- `POST /rotate` with `{"plan": "testdata/plans/basic.json"}` — run one batch

The same batch can be driven in-process:

```python
from rotgraph.plan import load_plan
from rotgraph.runner import execute

result = execute(load_plan("testdata/plans/basic.json"), journal_path="logs/basic.journal")
print(result.to_dict())
```

## Batch description

See `docs/rotation-plan.md` §2. A minimal example:

```json
{
  "batch": "2026-11-prod",
  "dwell": 120,
  "services": {
    "signer": {"endpoint": "https://signer.example.test", "secret": "svc-signer"},
    "objstore": {"endpoint": "https://objstore.example.test", "secret": "svc-objstore"}
  },
  "keys": [
    {"name": "objstore.prod", "service": "objstore", "slot": "prod",
     "current": {"id": "obj_old_1", "secret": "..."}},
    {"name": "signer.prod", "service": "signer", "slot": "prod",
     "depends_on": ["objstore.prod"],
     "current": {"id": "sig_old_1", "secret": "..."}}
  ],
  "consumers": [
    {"name": "billing", "endpoint": "http://billing.internal.test", "secret": "..."}
  ]
}
```

## Specification

The four documents under `docs/` are the source of truth for behaviour:

- `docs/rotation-plan.md` — config shape, dependencies, declaration order, cycles
- `docs/swap-window.md` — key lifecycle, the dual-live window, verification, retirement
- `docs/notify.md` — webhook headers, signature, timestamp window, retries, receipts
- `docs/recovery.md` — journal ordering, durability, redaction, restart semantics
