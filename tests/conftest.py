"""Shared fixtures for the rotseal test-suite."""

from __future__ import annotations

import json

import pytest

from rotseal import keys
from rotseal.util import active_path, ensure_archive


def write_records(path, start=1, count=100, level="info", message="entry"):
    """Write ``count`` NDJSON records into the active log file."""

    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        for offset in range(count):
            seq = start + offset
            record = {
                "seq": seq,
                "ts": f"2026-01-01T{(offset // 3600) % 24:02d}:"
                f"{(offset // 60) % 60:02d}:{offset % 60:02d}Z",
                "level": level,
                "msg": f"{message}-{seq}",
            }
            handle.write(json.dumps(record, ensure_ascii=False) + "\n")


@pytest.fixture
def archive(tmp_path):
    return ensure_archive(tmp_path / "archive")


@pytest.fixture
def keyring_file(tmp_path, monkeypatch):
    path = tmp_path / "keyring.pickle"
    keys.add_key(path, key_id="key-a")
    monkeypatch.setenv("ROTSEAL_KEYRING", str(path))
    monkeypatch.setenv("ROTSEAL_ACTIVE_KEY", "key-a")
    return path


@pytest.fixture
def keyring(keyring_file):
    return keys.load_keyring(keyring_file)


@pytest.fixture
def active(archive):
    return active_path(archive)
