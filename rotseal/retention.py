"""Retention: remove sealed segments that fell out of the retention window."""

from __future__ import annotations

import os
import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

from .util import (
    ensure_archive,
    entries_by_number,
    load_index,
    load_tombstones,
    parse_duration,
    parse_timestamp,
    save_index,
    save_tombstones,
    seal_filename,
    segment_filename,
    segments_dir,
    utcnow,
)


def _sealed_at(sidecar: Path) -> datetime:
    """Read the sealing time from the sidecar, never from filesystem metadata."""

    try:
        doc = json.loads(sidecar.read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read sidecar: {exc}") from exc
    if not isinstance(doc, dict) or not doc.get("sealed_at"):
        raise ValueError("sidecar has no sealed_at")
    try:
        moment = parse_timestamp(doc["sealed_at"])
    except Exception as exc:
        raise ValueError(f"invalid sealed_at: {doc.get('sealed_at')!r}") from exc
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=timezone.utc)
    return moment.astimezone(timezone.utc)


def _make_writable(directory: Path) -> None:
    try:
        os.chmod(directory, 0o755)
    except OSError:
        pass


def _make_writable_ok(directory: Path, segment: Path, sidecar: Path) -> bool:
    """Clear the read-only archive protection so a segment can be deleted."""

    try:
        os.chmod(segment, 0o644)
    except OSError:
        return False
    try:
        os.chmod(sidecar, 0o644)
    except OSError:
        pass
    _make_writable(directory)
    return True


def _tombstone_for(number: int, doc: dict, window: str, clock: datetime) -> dict:
    return {
        "number": int(number),
        "removed_at": clock.astimezone(timezone.utc)
        .isoformat()
        .replace("+00:00", "Z"),
        "window": window,
        "reason": "retention",
        "seq_start": doc.get("seq_start"),
        "seq_end": doc.get("seq_end"),
        "count": doc.get("count"),
        "key_id": doc.get("key_id"),
        "archive_id": doc.get("archive_id"),
        "tip": doc.get("tip"),
        "records_digest": doc.get("records_digest"),
        "sealed_at": doc.get("sealed_at"),
    }


def prune(archive, keep, now=None, dry_run=False) -> dict:
    """Delete sealed segments older than the ``keep`` window.

    ``keep`` accepts values such as ``30d`` or ``12h``.  Only sealed segments
    are considered; the active log file is never touched.  Returns a summary
    with the numbers of the removed segments.
    """

    archive = ensure_archive(archive)
    window = parse_duration(keep)
    clock = now or utcnow()
    if clock.tzinfo is None:
        clock = clock.replace(tzinfo=timezone.utc)
    clock = clock.astimezone(timezone.utc)
    directory = segments_dir(archive)
    index = load_index(archive)
    entries = entries_by_number(index)
    tombstones = load_tombstones(archive)

    removed = []
    reported = []
    for number in sorted(entries):
        segment = directory / segment_filename(number)
        sidecar = directory / seal_filename(number)
        if not segment.exists():
            continue
        if not sidecar.exists():
            reported.append(
                {
                    "segment": number,
                    "reason": "sidecar is missing; not eligible for retention",
                }
            )
            continue
        try:
            sealed_at = _sealed_at(sidecar)
        except ValueError as exc:
            reported.append({"segment": number, "reason": str(exc)})
            continue
        try:
            doc = json.loads(sidecar.read_text(encoding="utf-8"))
        except (OSError, UnicodeDecodeError, json.JSONDecodeError):
            reported.append(
                {"segment": number, "reason": "sidecar is not readable"}
            )
            continue
        age = clock - sealed_at
        if age < window:
            continue
        removed.append(number)
        if dry_run:
            continue
        if not _make_writable_ok(directory, segment, sidecar):
            reported.append(
                {"segment": number, "reason": "segment could not be made writable"}
            )
            continue

        tombstone = _tombstone_for(number, doc, str(keep), clock)
        tombstones.append(tombstone)
        embedded = index.setdefault("tombstones", [])
        embedded.append(tombstone)
        save_tombstones(archive, tombstones)
        save_index(archive, index)

        try:
            os.remove(segment)
        except OSError:
            reported.append(
                {"segment": number, "reason": "segment could not be deleted"}
            )
            continue
        try:
            os.remove(sidecar)
        except OSError:
            reported.append(
                {"segment": number, "reason": "sidecar could not be deleted"}
            )
        index["segments"] = [
            item
            for item in index["segments"]
            if int(item.get("number", -1)) != int(number)
        ]
        save_index(archive, index)

    if not dry_run:
        save_index(archive, index)
    return {
        "window": str(keep),
        "removed": removed,
        "reported": reported,
        "dry_run": bool(dry_run),
    }
