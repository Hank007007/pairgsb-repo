"""Filesystem, JSON and time helpers shared by the rotseal commands."""

from __future__ import annotations

import json
import os
import re
import tempfile
from datetime import datetime, timedelta, timezone
from pathlib import Path

SEGMENTS_DIRNAME = "segments"
INDEX_FILENAME = "index.json"
ACTIVE_FILENAME = "active.ndjson"
INDEX_VERSION = 1


class RotsealError(Exception):
    """Raised when an archive operation cannot be completed."""


# ---------------------------------------------------------------------------
# archive layout
# ---------------------------------------------------------------------------


def archive_root(path) -> Path:
    return Path(path)


def active_path(archive) -> Path:
    return Path(archive) / ACTIVE_FILENAME


def segments_dir(archive) -> Path:
    return Path(archive) / SEGMENTS_DIRNAME


def index_path(archive) -> Path:
    return Path(archive) / INDEX_FILENAME


def segment_filename(number: int) -> str:
    return f"segment-{int(number):06d}.ndjson"


def seal_filename(number: int) -> str:
    return f"segment-{int(number):06d}.seal"


def ensure_archive(archive) -> Path:
    root = Path(archive)
    (root / SEGMENTS_DIRNAME).mkdir(parents=True, exist_ok=True)
    return root


# ---------------------------------------------------------------------------
# time
# ---------------------------------------------------------------------------


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


def isoformat_utc(moment: datetime | None = None) -> str:
    moment = moment or utcnow()
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=timezone.utc)
    return moment.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


def parse_timestamp(value) -> datetime:
    if isinstance(value, datetime):
        return value
    text = str(value).strip()
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        return datetime.fromisoformat(text)
    except ValueError as exc:
        raise RotsealError(f"invalid timestamp: {value!r}") from exc


_DURATION_RE = re.compile(r"^\s*(\d+)\s*([smhdw])\s*$", re.IGNORECASE)
_DURATION_UNITS = {
    "s": "seconds",
    "m": "minutes",
    "h": "hours",
    "d": "days",
    "w": "weeks",
}


def parse_duration(value) -> timedelta:
    if isinstance(value, timedelta):
        return value
    match = _DURATION_RE.match(str(value))
    if not match:
        raise RotsealError(
            f"invalid duration: {value!r} (expected a value such as 90m, 12h, 30d)"
        )
    amount, unit = match.groups()
    return timedelta(**{_DURATION_UNITS[unit.lower()]: int(amount)})


# ---------------------------------------------------------------------------
# JSON
# ---------------------------------------------------------------------------


def read_json(path) -> object:
    path = Path(path)
    try:
        with path.open("r", encoding="utf-8") as handle:
            return json.load(handle)
    except FileNotFoundError as exc:
        raise RotsealError(f"missing file: {path}") from exc
    except json.JSONDecodeError as exc:
        raise RotsealError(f"invalid JSON in {path}: {exc}") from exc


def write_json_atomic(path, payload) -> None:
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    handle = tempfile.NamedTemporaryFile(
        "w",
        encoding="utf-8",
        dir=str(path.parent),
        prefix=path.name + ".",
        suffix=".tmp",
        delete=False,
    )
    try:
        with handle:
            json.dump(payload, handle, ensure_ascii=False, indent=2, sort_keys=True)
            handle.write("\n")
        os.replace(handle.name, path)
    except BaseException:
        try:
            os.unlink(handle.name)
        except OSError:
            pass
        raise


# ---------------------------------------------------------------------------
# digests
# ---------------------------------------------------------------------------


def sha256_bytes(data: bytes) -> str:
    import hashlib

    return hashlib.sha256(data).hexdigest()


def sha256_file(path) -> str:
    import hashlib

    digest = hashlib.sha256()
    with Path(path).open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


# ---------------------------------------------------------------------------
# permissions
# ---------------------------------------------------------------------------


def set_readonly(path, readonly: bool = True) -> None:
    mode = 0o444 if readonly else 0o644
    try:
        os.chmod(path, mode)
    except OSError:
        pass


def set_segments_readonly(archive, readonly: bool = True) -> None:
    directory = segments_dir(archive)
    if not directory.exists():
        return
    for child in directory.iterdir():
        if child.is_file():
            set_readonly(child, readonly)
    try:
        os.chmod(directory, 0o555 if readonly else 0o755)
    except OSError:
        pass


# ---------------------------------------------------------------------------
# archive index
# ---------------------------------------------------------------------------


def empty_index() -> dict:
    return {"version": INDEX_VERSION, "segments": []}


def load_index(archive) -> dict:
    path = index_path(archive)
    if not path.exists():
        return empty_index()
    data = read_json(path)
    if not isinstance(data, dict) or not isinstance(data.get("segments"), list):
        raise RotsealError(f"malformed archive index: {path}")
    data.setdefault("version", INDEX_VERSION)
    return data


def save_index(archive, index: dict) -> None:
    write_json_atomic(index_path(archive), index)


def entries_by_number(index: dict) -> dict:
    result = {}
    for entry in index.get("segments", []):
        try:
            number = int(entry["number"])
        except (KeyError, TypeError, ValueError) as exc:
            raise RotsealError(f"malformed segment entry in index: {entry!r}") from exc
        result[number] = entry
    return result


def next_segment_number(index: dict) -> int:
    numbers = [int(entry.get("number", 0)) for entry in index.get("segments", [])]
    return max(numbers, default=0) + 1


def append_entry(index: dict, entry: dict) -> dict:
    for existing in index["segments"]:
        if int(existing.get("number", -1)) == int(entry["number"]):
            existing.update(entry)
            return existing
    index["segments"].append(entry)
    index["segments"].sort(key=lambda item: int(item.get("number", 0)))
    return entry
