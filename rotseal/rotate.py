"""Rotation: close the active NDJSON log into archive segments."""

from __future__ import annotations

import json
import os
from datetime import datetime, timedelta
from pathlib import Path

from .util import (
    RotsealError,
    active_path,
    append_entry,
    ensure_archive,
    isoformat_utc,
    load_index,
    next_segment_number,
    parse_duration,
    parse_timestamp,
    save_index,
    segment_filename,
    segments_dir,
    utcnow,
)

DEFAULT_CHUNK_BYTES = 1024 * 1024


def _make_writable(directory: Path) -> None:
    try:
        os.chmod(directory, 0o755)
    except OSError:
        pass


def _split_bytes(data: bytes, max_bytes: int) -> list:
    """Cut the active file into chunks of at most ``max_bytes`` bytes."""

    chunks = []
    for offset in range(0, len(data), max_bytes):
        chunk = data[offset : offset + max_bytes]
        if chunk:
            chunks.append(chunk)
    return chunks


def _wall_clock(value) -> datetime:
    """Wall-clock reading of a timestamp.

    The clock reading is taken as written; the offset in the text is dropped
    and no conversion to another zone happens.
    """

    return parse_timestamp(value).replace(tzinfo=None)


def _split_records(data: bytes, every: timedelta, now: datetime) -> list:
    """Split records into consecutive windows of ``every``.

    Records are consumed in file order.  A window closes when the wall-clock
    reading moves past the window length.  A reading that steps backwards
    closes the window as well, and the following window begins with the
    record seen last.
    """

    text = data.decode("utf-8", errors="ignore")
    groups = []
    current = []
    started = None
    previous = None
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped:
            continue
        try:
            record = json.loads(stripped)
        except json.JSONDecodeError:
            continue
        stamp = record.get("ts")
        try:
            moment = _wall_clock(stamp) if stamp else now
        except RotsealError:
            moment = now

        if started is None:
            started = moment
        elif previous is not None and moment < previous:
            groups.append(current)
            current = current[-1:]
            started = moment
        elif previous is not None and moment - started > every and current:
            groups.append(current)
            current = []
            started = moment

        current.append(stripped)
        previous = moment

    if current:
        groups.append(current)
    return [("\n".join(group) + "\n").encode("utf-8") for group in groups]


def rotate(
    archive,
    max_bytes=None,
    every=None,
    now=None,
    reason: str = "manual",
) -> list:
    """Close the active log file into one or more archive segments.

    With ``max_bytes`` the file is cut into chunks of that many bytes.  With
    ``every`` records are grouped into windows of that length.  Without
    either the whole file becomes a single segment.
    """

    archive = ensure_archive(archive)
    source = active_path(archive)
    if not source.exists():
        raise RotsealError(f"no active log file at {source}")
    data = source.read_bytes()
    if not data:
        source.unlink()
        return []

    clock = now or datetime.now()
    chunk_size = max_bytes or DEFAULT_CHUNK_BYTES
    if every:
        interval = parse_duration(every)
        chunks = _split_records(data, interval, clock)
        mode = "time"
    else:
        try:
            limit = int(chunk_size)
        except (TypeError, ValueError) as exc:
            raise RotsealError(f"invalid byte limit: {chunk_size!r}") from exc
        if limit <= 0:
            raise RotsealError("byte limit must be positive")
        chunks = _split_bytes(data, limit)
        mode = "size"

    directory = segments_dir(archive)
    _make_writable(directory)
    index = load_index(archive)
    created = []
    stamp = isoformat_utc(utcnow())
    for chunk in chunks:
        if not chunk.strip():
            continue
        number = next_segment_number(index)
        path = directory / segment_filename(number)
        with path.open("wb") as handle:
            handle.write(chunk)
        try:
            os.chmod(path, 0o644)
        except OSError:
            pass
        entry = {
            "number": number,
            "file": path.name,
            "mode": mode,
            "reason": reason,
            "bytes": len(chunk),
            "rotated_at": stamp,
        }
        append_entry(index, entry)
        created.append(entry)
    save_index(archive, index)
    try:
        source.unlink()
    except OSError as exc:
        raise RotsealError(f"cannot remove the active log file: {exc}") from exc
    return created
