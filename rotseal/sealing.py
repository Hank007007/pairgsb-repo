"""Segment sealing: turn a closed NDJSON segment into an immutable archive block.

Sealing computes the digests and counters that describe a segment, signs the
block with the active key and writes a JSON sidecar next to the segment.  The
sidecar is stored next to the segment so the pair can be moved or copied as a
unit.
"""

from __future__ import annotations

import json
from pathlib import Path

from . import keys as keys_mod
from .util import (
    RotsealError,
    ensure_archive,
    isoformat_utc,
    seal_filename,
    segment_filename,
    segments_dir,
    sha256_bytes,
    sha256_file,
    utcnow,
    write_json_atomic,
)

SEAL_VERSION = 1


def _signed_payload(records_digest: str) -> bytes:
    """Bytes handed to the signature function for a sealed segment."""

    return str(records_digest).encode("utf-8")


def _sign(secret: bytes, body: bytes) -> str:
    import hashlib
    import hmac

    return hmac.new(secret, body, hashlib.sha256).hexdigest()


def tip_for(number: int, seal_doc: dict) -> str:
    """Chain tip produced by a sealed segment.

    The tip binds the segment number, its sequence range and the digest of
    its records so that any later segment can name its predecessor.
    """

    material = json.dumps(
        {
            "segment": int(number),
            "seq_start": seal_doc.get("seq_start"),
            "seq_end": seal_doc.get("seq_end"),
            "records_digest": seal_doc.get("records_digest"),
        },
        sort_keys=True,
        separators=(",", ":"),
    )
    return sha256_bytes(material.encode("utf-8"))


def compute_digest(records: list) -> str:
    """Digest over the log records of a segment."""

    parts = []
    for record in records:
        parts.append(
            json.dumps(record, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
        )
    return sha256_bytes("\n".join(parts).encode("utf-8"))


def read_records(path):
    """Read an NDJSON segment into memory.

    Lines that do not decode are left out so a partially written tail does
    not stop the segment from being processed.
    """

    records = []
    text = Path(path).read_text(encoding="utf-8", errors="ignore")
    for line in text.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            records.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return records


def inspect_segment(path) -> dict:
    """Describe a segment file on disk."""

    path = Path(path)
    if not path.exists():
        raise RotsealError(f"missing segment: {path}")
    records = read_records(path)
    seqs = [int(record.get("seq")) for record in records if "seq" in record]
    return {
        "count": len(records),
        "sealed_length": path.stat().st_size,
        "records_digest": compute_digest(records),
        "seq_start": seqs[0] if seqs else None,
        "seq_end": seqs[-1] if seqs else None,
        "first_ts": records[0].get("ts") if records else None,
        "last_ts": records[-1].get("ts") if records else None,
    }


def seal_segment(
    archive,
    number: int,
    keyring: dict,
    key_id=None,
    prev_tip=None,
    created_at=None,
) -> dict:
    """Seal ``segment-<number>.ndjson`` and write its ``.seal`` sidecar."""

    archive = ensure_archive(archive)
    directory = segments_dir(archive)
    segment = directory / segment_filename(number)
    if not segment.exists():
        raise RotsealError(f"missing segment: {segment}")

    info = inspect_segment(segment)
    if info["count"] == 0:
        raise RotsealError(f"cannot seal empty segment: {segment}")

    if key_id is None:
        entry = keys_mod.active_key(keyring)
    else:
        entry = keys_mod.find_key(keyring, key_id)
        if entry is None:
            raise RotsealError(f"unknown key id: {key_id!r}")

    signature = _sign(
        keys_mod.secret_bytes(entry), _signed_payload(info["records_digest"])
    )

    seal_doc = {
        "version": SEAL_VERSION,
        "segment": int(number),
        "segment_file": segment.name,
        "seq_start": info["seq_start"],
        "seq_end": info["seq_end"],
        "count": info["count"],
        "prev_tip": prev_tip,
        "records_digest": info["records_digest"],
        "sealed_length": info["sealed_length"],
        "first_ts": info["first_ts"],
        "last_ts": info["last_ts"],
        "key_id": str(entry["id"]),
        "alg": "hmac-sha256",
        "sealed_at": isoformat_utc(created_at or utcnow()),
        "signature": signature,
    }
    seal_doc["tip"] = tip_for(number, seal_doc)
    sidecar = directory / seal_filename(number)
    write_json_atomic(sidecar, seal_doc)
    return seal_doc


def seal(archive, keyring: dict, key_id=None) -> list:
    """Seal every unsealed segment in the archive.

    Segments are sealed in numeric order.  A segment without a sidecar takes
    the tip of its immediate predecessor as ``prev_tip``.
    """

    from .util import entries_by_number, load_index, save_index

    archive = ensure_archive(archive)
    index = load_index(archive)
    entries = entries_by_number(index)
    directory = segments_dir(archive)

    sealed = []
    tip = None
    for number in sorted(entries):
        sidecar = directory / seal_filename(number)
        if sidecar.exists():
            doc = json.loads(sidecar.read_text(encoding="utf-8"))
            tip = tip_for(number, doc)
            continue
        if not (directory / segment_filename(number)).exists():
            continue
        if not read_records(directory / segment_filename(number)):
            continue
        doc = seal_segment(
            archive, number, keyring, key_id=key_id, prev_tip=tip
        )
        entries[number].update(
            {
                "sealed": True,
                "key_id": doc["key_id"],
                "count": doc["count"],
                "seq_start": doc["seq_start"],
                "seq_end": doc["seq_end"],
                "sealed_length": doc["sealed_length"],
            }
        )
        tip = tip_for(number, doc)
        sealed.append(number)
    save_index(archive, index)
    return sealed


def file_digest(path) -> str:
    return sha256_file(path)
