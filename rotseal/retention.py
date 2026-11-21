"""Retention: remove sealed segments that fell out of the retention window."""

from __future__ import annotations

import os
from datetime import datetime, timedelta
from pathlib import Path

from .util import (
    ensure_archive,
    entries_by_number,
    load_index,
    parse_duration,
    save_index,
    seal_filename,
    segment_filename,
    segments_dir,
    utcnow,
)


def _segment_age(segment: Path, now: datetime) -> timedelta:
    """How old a segment is, measured from the last time it was written."""

    mtime = datetime.fromtimestamp(segment.stat().st_mtime, tz=now.tzinfo)
    return now - mtime


def _make_writable(directory: Path) -> None:
    try:
        os.chmod(directory, 0o755)
    except OSError:
        pass


def _make_writable_ok(directory: Path, segment: Path) -> bool:
    """Clear the read-only archive protection so a segment can be deleted."""

    try:
        os.chmod(segment, 0o644)
    except OSError:
        return False
    _make_writable(directory)
    return True


def prune(archive, keep, now=None, dry_run=False) -> dict:
    """Delete sealed segments older than the ``keep`` window.

    ``keep`` accepts values such as ``30d`` or ``12h``.  Only sealed segments
    are considered; the active log file is never touched.  Returns a summary
    with the numbers of the removed segments.
    """

    archive = ensure_archive(archive)
    window = parse_duration(keep)
    clock = now or utcnow()
    directory = segments_dir(archive)
    index = load_index(archive)
    entries = entries_by_number(index)

    removed = []
    for number in sorted(entries):
        segment = directory / segment_filename(number)
        sidecar = directory / seal_filename(number)
        if not segment.exists():
            continue
        if not sidecar.exists():
            continue
        try:
            age = _segment_age(segment, clock)
        except OSError:
            continue
        if age < window:
            continue
        removed.append(number)
        if dry_run:
            continue
        if not _make_writable_ok(directory, segment):
            continue
        try:
            os.remove(segment)
        except OSError:
            continue
        try:
            os.remove(sidecar)
        except OSError:
            pass
        index["segments"] = [
            item
            for item in index["segments"]
            if int(item.get("number", -1)) != int(number)
        ]

    if not dry_run:
        save_index(archive, index)
    return {
        "window": str(keep),
        "removed": removed,
        "dry_run": bool(dry_run),
    }
