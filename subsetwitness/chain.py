"""Append-only main hash chain of audit records.

Each line is one JSON object with the shape::

    {"seq": 1, "kind": "event", "ts": ..., "actor": ..., "action": ...}
    {"seq": 9, "kind": "commit", "period_id": ..., ...}

Event records carry ``prev``/``hash`` chain fields once sealed; commit
records do too.  The chain is the source of truth that a disclosure
package is checked against.
"""

from __future__ import annotations

import hashlib
import json
import os

GENESIS_HASH = "0" * 64
CHAIN_TAG = b"SWC1"


def canonical(record: dict) -> str:
    return json.dumps(record, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def record_hash(prev_hash: str, record: dict) -> str:
    """Hash of one chain record, excluding its own hash field."""
    body = {key: value for key, value in record.items() if key != "hash"}
    digest = hashlib.sha256()
    digest.update(CHAIN_TAG)
    digest.update(bytes.fromhex(prev_hash))
    digest.update(canonical(body).encode("utf-8"))
    return digest.hexdigest()


def seal(records: list[dict], start_prev: str = GENESIS_HASH) -> list[dict]:
    """Return ``records`` with prev/hash fields filled in."""
    sealed: list[dict] = []
    prev = start_prev
    for index, record in enumerate(records, start=1):
        entry = dict(record)
        entry.setdefault("seq", index)
        entry["prev"] = prev
        entry["hash"] = record_hash(prev, entry)
        prev = entry["hash"]
        sealed.append(entry)
    return sealed


def read_chain(path: str) -> list[dict]:
    """Read a chain file into a list of records.

    Records are joined with the chain fields stripped, matching how the
    writer emitted them (minus ``prev``/``hash``).
    """
    with open(path, "r", encoding="utf-8") as handle:
        text = handle.read()
    if not text.strip():
        return []
    return json.loads(text)


def write_chain(path: str, records: list[dict]) -> None:
    directory = os.path.dirname(os.path.abspath(path))
    if directory:
        os.makedirs(directory, exist_ok=True)
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        json.dump(records, handle, ensure_ascii=False, indent=1)
        handle.write("\n")


def append_commit(path: str, commitment: dict) -> dict:
    """Append a commit record to the chain file."""
    records = read_chain(path)
    prev = records[-1]["hash"] if records else GENESIS_HASH
    entry = dict(commitment)
    entry["kind"] = "commit"
    entry["prev"] = prev
    entry["hash"] = record_hash(prev, entry)
    records.append(entry)
    write_chain(path, records)
    return entry


def find_commitment(records: list[dict], period_id: str) -> dict | None:
    """Return the latest commit record for ``period_id``.

    A period may be sealed more than once; the most recent commit on
    the chain is the authoritative one.
    """
    found = None
    for record in records:
        if record.get("kind") == "commit" and record.get("period_id") == period_id:
            found = record
    return found


def verify_chain(records: list[dict]) -> tuple[bool, str | None]:
    """Walk the chain and report the first broken link."""
    prev = GENESIS_HASH
    for record in records:
        expected = record_hash(prev, record)
        if record.get("hash") != expected or record.get("prev") != prev:
            return False, str(record.get("seq"))
        prev = record["hash"]
    return True, None
