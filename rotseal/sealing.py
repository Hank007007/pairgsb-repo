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
    load_index,
    load_tombstones,
    seal_filename,
    segment_filename,
    segments_dir,
    sha256_bytes,
    sha256_file,
    utcnow,
    write_json_atomic,
)

SEAL_VERSION = 1

SIGNED_FIELDS = (
    "version",
    "archive_id",
    "segment",
    "segment_file",
    "seq_start",
    "seq_end",
    "count",
    "prev_tip",
    "records_digest",
    "sealed_length",
    "first_ts",
    "last_ts",
    "key_id",
    "alg",
    "sealed_at",
    "tip",
)


def signature_payload(seal_doc: dict) -> bytes:
    """Canonical bytes covered by a segment signature.

    Every field verification or retention relies on is included.  The
    serialisation is independent of JSON key order and whitespace.
    """

    material = {name: seal_doc.get(name) for name in SIGNED_FIELDS}
    return json.dumps(
        material, sort_keys=True, separators=(",", ":"), ensure_ascii=False
    ).encode("utf-8")


def _sign(secret: bytes, body: bytes) -> str:
    import hashlib
    import hmac

    return hmac.new(secret, body, hashlib.sha256).hexdigest()


def signature_for(entry: dict, seal_doc: dict) -> str:
    return _sign(keys_mod.secret_bytes(entry), signature_payload(seal_doc))


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
    """Read and strictly validate an NDJSON segment.

    Invalid UTF-8, invalid JSON and blank lines are errors.  Silently dropping
    them would make the sidecar's ``count`` disagree with the amount of data
    an auditor sees in the file.
    """

    path = Path(path)
    try:
        raw = path.read_bytes()
    except OSError as exc:
        raise RotsealError(f"cannot read segment {path}: {exc}") from exc
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise RotsealError(f"segment {path} is not valid UTF-8: {exc}") from exc

    records = []
    for number, line in enumerate(text.splitlines(), start=1):
        if not line.strip():
            raise RotsealError(f"segment {path} line {number} is blank")
        try:
            record = json.loads(line)
        except json.JSONDecodeError as exc:
            raise RotsealError(
                f"segment {path} line {number} is not valid JSON: {exc.msg}"
            ) from exc
        if not isinstance(record, dict):
            raise RotsealError(
                f"segment {path} line {number} is not a JSON object"
            )
        records.append(record)
    return records


def inspect_segment(path) -> dict:
    """Describe a segment file on disk."""

    path = Path(path)
    if not path.exists():
        raise RotsealError(f"missing segment: {path}")
    records = read_records(path)
    seqs = []
    for number, record in enumerate(records, start=1):
        if "seq" not in record:
            raise RotsealError(f"segment {path} line {number} has no seq field")
        try:
            seq = int(record["seq"])
        except (TypeError, ValueError) as exc:
            raise RotsealError(
                f"segment {path} line {number} has a non-integer seq"
            ) from exc
        if seqs and seq != seqs[-1] + 1:
            raise RotsealError(
                f"segment {path} sequence {seqs[-1]} is not followed by {seq}"
            )
        seqs.append(seq)
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

    archive_id = load_index(archive).get("archive_id")

    if key_id is None:
        entry = keys_mod.active_key(keyring)
    else:
        entry = keys_mod.find_key(keyring, key_id)
        if entry is None:
            raise RotsealError(f"unknown key id: {key_id!r}")

    if prev_tip is None and int(number) > 1:
        predecessor = directory / seal_filename(int(number) - 1)
        if predecessor.exists():
            try:
                predecessor_doc = json.loads(
                    predecessor.read_text(encoding="utf-8")
                )
            except (OSError, json.JSONDecodeError):
                predecessor_doc = None
            if isinstance(predecessor_doc, dict):
                prev_tip = tip_for(int(number) - 1, predecessor_doc)

    seal_doc = {
        "version": SEAL_VERSION,
        "archive_id": archive_id,
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
    }
    seal_doc["tip"] = tip_for(number, seal_doc)
    seal_doc["signature"] = signature_for(entry, seal_doc)
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
    tip_by_number = {}
    for item in load_tombstones(archive):
        try:
            tip_by_number[int(item["number"])] = item.get("tip")
        except (KeyError, TypeError, ValueError):
            continue
    for number in sorted(entries):
        sidecar = directory / seal_filename(number)
        if sidecar.exists():
            try:
                doc = json.loads(sidecar.read_text(encoding="utf-8"))
            except (OSError, json.JSONDecodeError) as exc:
                raise RotsealError(
                    f"cannot read sidecar {sidecar}: {exc}"
                ) from exc
            tip_by_number[number] = tip_for(number, doc)
            continue
        if not (directory / segment_filename(number)).exists():
            continue
        if not read_records(directory / segment_filename(number)):
            continue
        prev_tip = None if number <= 1 else tip_by_number.get(number - 1)
        doc = seal_segment(archive, number, keyring, key_id=key_id, prev_tip=prev_tip)
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
        tip_by_number[number] = tip_for(number, doc)
        sealed.append(number)
    save_index(archive, index)
    return sealed


def file_digest(path) -> str:
    return sha256_file(path)
