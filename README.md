# syslogfold

syslogfold is the intake layer between network devices / applications and the
tenant channels of the security team's collector. It accepts one RFC 5424
message at a time, validates it, assigns it to a tenant, de-duplicates it, and
forwards it downstream as a single CEF line.

```
wire message ──► PRI / VERSION ──► header + SD + MSG ──► routing ──► dedup ──► CEF
```

## Layout

| path | contents |
|---|---|
| `syslogfold/pri.py` | `parse_pri`: PRI and VERSION |
| `syslogfold/message.py` | `parse`: timestamp, header fields, structured data, body |
| `syslogfold/sd.py` | `parse_sd`, `escape_param`: structured data |
| `syslogfold/route.py` | `tenant_for`, `dedup_key`: routing and de-duplication |
| `syslogfold/emit.py` | `emit_cef`: downstream CEF rendering |
| `syslogfold/pipeline.py` | `ingest`: ties the stages together and reports errors |
| `examples/replay.py` | replays `fixtures/messages/` and prints fields plus CEF |
| `docs/` | the rules this layer enforces |
| `tests/` | pytest suite |

## Running

```
python -m pytest -q -p no:cacheprovider
python examples/replay.py
```

Only the Python standard library is required.

## Documents

- `docs/rfc5424-profile.md` — what a message must look like, field by field.
- `docs/routing.md` — how a tenant is chosen, what counts as a duplicate, and
  how a record is rendered as CEF.
- `docs/limits.md` — size limits.
- `docs/incidents.md` — production history that shaped the rules.
- `docs/design.md` — the stages of the pipeline and the contract of each.

## Input model

`ingest(raw, source=..., routes=..., seen=...)` takes the raw wire text (or
bytes) and the sender address. It returns a dictionary:

- accepted: `ok=True`, `tenant`, `key`, `message`, `cef`
- duplicate: `ok=False`, `error` with `field="dedup"`
- rejected: `error` with the failing `field` and a `reason`

`seen` is an optional set the caller keeps between calls; it is used for
de-duplication within one collector run.
