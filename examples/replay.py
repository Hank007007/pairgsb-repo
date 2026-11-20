#!/usr/bin/env python3
"""Replay captured broker payloads through the decode boundary.

Reads every ``*.msgpack`` file in ``fixtures/payloads/`` and prints what the
consumer makes of it: either the handler-facing mapping or the rejection
reason.  The expected outcome for each payload is described in
``docs/payload-schema.md``.
"""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from payloadcomb import Consumer, decode  # noqa: E402
from tests.schemas import (  # noqa: E402
    ANY,
    BLOB,
    EXPORT,
    INT_LIST,
    JOB,
    LOCALISED,
    METRIC,
    SEQUENCE,
)

FIXTURES = ROOT / "fixtures" / "payloads"

# fixture name -> (label, schema, mode)
#   mode "one-shot": decode with a fresh budget
#   mode "stream":   feed through one Consumer shared with its predecessor
SCHEMAS = {
    "job_ok": ("job", JOB, "one-shot"),
    "job_missing_optional": ("job", JOB, "one-shot"),
    "job_missing_required": ("job", JOB, "one-shot"),
    "dup_key": ("job", JOB, "one-shot"),
    "extra_key": ("job", JOB, "one-shot"),
    "int_key": ("job", JOB, "one-shot"),
    "bad_utf8": ("job", JOB, "one-shot"),
    "bad_utf8_in_list": ("job", JOB, "one-shot"),
    "int_out_of_range": ("job", JOB, "one-shot"),
    "bool_as_int": ("job", JOB, "one-shot"),
    "pickle_ext": ("job", JOB, "one-shot"),
    "unknown_ext": ("job", JOB, "one-shot"),
    "unknown_ext_any": ("blob", BLOB, "one-shot"),
    "dup_key_nfc": ("localised", LOCALISED, "one-shot"),
    "uint64_unbounded": ("sequence", SEQUENCE, "one-shot"),
    "nan_score": ("metric", METRIC, "one-shot"),
    "inf_score": ("metric", METRIC, "one-shot"),
    "neg_inf_score": ("metric", METRIC, "one-shot"),
    "pickle_obj": ("metric", METRIC, "one-shot"),
    "too_deep": ("any", ANY, "one-shot"),
    "deep_recursion": ("any", ANY, "one-shot"),
    "too_large": ("any", ANY, "one-shot"),
    "too_large_invalid": ("int-list", INT_LIST, "one-shot"),
    "export_ok": ("export", EXPORT, "one-shot"),
    "seq_1": ("int-list", INT_LIST, "stream"),
    "seq_2": ("int-list", INT_LIST, "stream"),
}


def render(value, limit=160):
    text = repr(value)
    text = text.encode("unicode_escape").decode("ascii")
    if len(text) > limit:
        text = text[:limit] + f"... ({len(text)} chars)"
    return text


def main():
    if not FIXTURES.is_dir():
        print(f"no fixture directory at {FIXTURES}", file=sys.stderr)
        return 2

    paths = sorted(FIXTURES.glob("*.msgpack"))
    if not paths:
        print(f"no payloads in {FIXTURES}", file=sys.stderr)
        return 2

    streams = {}
    accepted = 0
    rejected = 0
    for path in paths:
        name = path.stem
        label, schema, mode = SCHEMAS.get(name, ("job", JOB, "one-shot"))
        data = path.read_bytes()
        print(f"--- {name}  ({len(data)} bytes, schema={label}, {mode})")
        try:
            if mode == "stream":
                consumer = streams.get(label)
                if consumer is None:
                    consumer = streams[label] = Consumer(schema)
                result = consumer.handle(data)
            else:
                result = decode(data, schema)
        except Exception as exc:  # noqa: BLE001 - replay reports every outcome
            rejected += 1
            print(f"    REJECTED  {type(exc).__name__}: {exc}")
        else:
            accepted += 1
            print(f"    ACCEPTED  {render(result)}")
        print()

    print(f"{len(paths)} payload(s) replayed, {accepted} accepted, {rejected} rejected")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
