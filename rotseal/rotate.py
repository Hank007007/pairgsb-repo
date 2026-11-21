"""Rotation: close the active NDJSON log into archive segments."""

from __future__ import annotations

import json
import os
from datetime import datetime, timedelta, timezone
from pathlib import Path

from .util import (
    RotsealError,
    active_path,
    append_entry,
    ensure_archive,
    isoformat_utc,
    load_index,
    load_tombstones,
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


def _decode_record_lines(data: bytes) -> list:
    """Decode the active log into whole, validated NDJSON lines.

    Rotation must never split a multibyte character or silently discard a
    line it does not understand.  Validation happens before any segment is
    written, so a malformed active log leaves the archive untouched.
    """

    lines = []
    parts = data.split(b"\n")
    for number, raw in enumerate(parts, start=1):
        if raw.endswith(b"\r"):
            raw = raw[:-1]
        if not raw:
            if number == len(parts):
                continue
            raise RotsealError(f"active log line {number} is blank")
        if not raw.strip():
            raise RotsealError(f"active log line {number} is blank")
        try:
            text = raw.decode("utf-8")
        except UnicodeDecodeError as exc:
            raise RotsealError(
                f"active log line {number} is not valid UTF-8: {exc}"
            ) from exc
        try:
            record = json.loads(text)
        except json.JSONDecodeError as exc:
            raise RotsealError(
                f"active log line {number} is not valid JSON: {exc.msg}"
            ) from exc
        if not isinstance(record, dict):
            raise RotsealError(f"active log line {number} is not a JSON object")
        if "seq" not in record:
            raise RotsealError(f"active log line {number} has no seq field")
        try:
            int(record["seq"])
        except (TypeError, ValueError) as exc:
            raise RotsealError(
                f"active log line {number} has a non-integer seq"
            ) from exc
        lines.append((raw + b"\n", record))
    return lines


def _instant(value) -> datetime:
    """Return a timestamp as an absolute instant.

    Offsets are meaningful.  Dropping them makes the repeated hour of a
    daylight-saving fall-back look like time moving backwards.
    """

    moment = parse_timestamp(value)
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=timezone.utc)
    return moment.astimezone(timezone.utc)


def _split_by_size(lines: list, max_bytes: int) -> list:
    """Group whole records into chunks of at most ``max_bytes`` when possible."""

    chunks = []
    current = []
    size = 0
    for raw, _record in lines:
        if current and size + len(raw) > max_bytes:
            chunks.append(b"".join(current))
            current = []
            size = 0
        current.append(raw)
        size += len(raw)
    if current:
        chunks.append(b"".join(current))
    return chunks


def _split_records(lines: list, every: timedelta) -> list:
    """Split whole records into consecutive windows of ``every``.

    The first record of a new window belongs to that window only.  Comparisons
    use absolute instants, so a local clock that repeats an hour does not
    duplicate or misorder records.
    """

    groups = []
    current = []
    started = None
    previous = None
    for raw, record in lines:
        stamp = record.get("ts")
        if stamp is None:
            raise RotsealError("cannot rotate by time: record has no ts field")
        try:
            moment = _instant(stamp)
        except RotsealError as exc:
            raise RotsealError(f"record has an invalid timestamp: {stamp!r}") from exc

        if started is None:
            started = moment
        elif previous is not None and moment < previous:
            groups.append(current)
            current = []
            started = moment
        elif current and moment - started >= every:
            groups.append(current)
            current = []
            started = moment

        current.append(raw)
        previous = moment

    if current:
        groups.append(current)
    return [b"".join(group) for group in groups]


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

    chunk_size = max_bytes or DEFAULT_CHUNK_BYTES
    lines = _decode_record_lines(data)
    if every:
        interval = parse_duration(every)
        chunks = _split_records(lines, interval)
        mode = "time"
    else:
        try:
            limit = int(chunk_size)
        except (TypeError, ValueError) as exc:
            raise RotsealError(f"invalid byte limit: {chunk_size!r}") from exc
        if limit <= 0:
            raise RotsealError("byte limit must be positive")
        chunks = _split_by_size(lines, limit)
        mode = "size"

    directory = segments_dir(archive)
    _make_writable(directory)
    index = load_index(archive)
    tombstones = load_tombstones(archive)
    created = []
    stamp = isoformat_utc(utcnow())
    for chunk in chunks:
        if not chunk.strip():
            continue
        number = next_segment_number(
            index, directory=directory, tombstones=tombstones
        )
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
