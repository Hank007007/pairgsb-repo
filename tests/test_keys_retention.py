"""Tests for keyring handling and the retention policy."""

from __future__ import annotations

import os
from datetime import timedelta

from rotseal import keys, prune, seal, seal_segment, verify_chain
from rotseal.rotate import rotate
from rotseal.util import (
    load_index,
    seal_filename,
    segment_filename,
    segments_dir,
    utcnow,
)

from conftest import write_records


def test_keyring_add_and_list(keyring_file):
    keys.add_key(keyring_file, key_id="key-b")
    ring = keys.load_keyring(keyring_file)
    assert keys.key_ids(ring) == ["key-a", "key-b"]
    assert ring["active"] == "key-b"
    assert keys.find_key(ring, "key-a") is not None


def test_active_key_follows_environment(keyring_file, monkeypatch):
    keys.add_key(keyring_file, key_id="key-b")
    ring = keys.load_keyring(keyring_file)
    monkeypatch.setenv("ROTSEAL_ACTIVE_KEY", "key-a")
    assert keys.active_key(ring)["id"] == "key-a"


def test_missing_keyring_env_is_an_error(tmp_path, monkeypatch):
    monkeypatch.delenv("ROTSEAL_KEYRING", raising=False)
    import pytest

    with pytest.raises(keys.RotsealError):
        keys.keyring_path(None)


def test_seal_and_verify_with_explicit_key(archive, keyring, active):
    write_records(active, start=1, count=8)
    rotate(archive)
    seal(archive, keyring, key_id="key-a")

    report = verify_chain(archive, keyring, from_segment=0)
    assert report.ok, report.failures


def test_prune_keeps_recent_segments(archive, keyring, active):
    write_records(active, start=1, count=8)
    rotate(archive)
    seal(archive, keyring)

    result = prune(archive, "30d")
    assert result["removed"] == []
    report = verify_chain(archive, keyring, from_segment=0)
    assert report.ok, report.failures
    assert len(report.checked) == 1


def test_prune_dry_run_reports_without_deleting(archive, keyring, active):
    write_records(active, start=1, count=8)
    rotate(archive)
    old = utcnow() - timedelta(days=60)
    seal_segment(archive, 1, keyring, created_at=old)
    segment = segments_dir(archive) / segment_filename(1)
    os.utime(segment, (old.timestamp(), old.timestamp()))

    result = prune(archive, "30d", dry_run=True)
    assert result["removed"] == [1]
    assert segment.exists()

    report = verify_chain(archive, keyring, from_segment=0)
    assert report.ok, report.failures


def test_prune_removes_expired_segment(archive, keyring, active):
    write_records(active, start=1, count=8)
    rotate(archive)
    old = utcnow() - timedelta(days=60)
    seal_segment(archive, 1, keyring, created_at=old)
    segment = segments_dir(archive) / segment_filename(1)
    os.utime(segment, (old.timestamp(), old.timestamp()))

    result = prune(archive, "30d")
    assert result["removed"] == [1]
    assert not segment.exists()
    assert not (segments_dir(archive) / seal_filename(1)).exists()


def test_prune_removes_segment_from_index(archive, keyring, active):
    write_records(active, start=1, count=8)
    rotate(archive)
    old = utcnow() - timedelta(days=60)
    seal_segment(archive, 1, keyring, created_at=old)
    segment = segments_dir(archive) / segment_filename(1)
    os.utime(segment, (old.timestamp(), old.timestamp()))

    prune(archive, "30d")

    index = load_index(archive)
    assert index["segments"] == []


def test_prune_skips_unsealed_segments(archive, keyring, active):
    write_records(active, start=1, count=8)
    rotate(archive)

    segment = segments_dir(archive) / segment_filename(1)
    old = utcnow() - timedelta(days=60)
    os.utime(segment, (old.timestamp(), old.timestamp()))

    result = prune(archive, "30d")
    assert result["removed"] == []
    assert segment.exists()


def test_prune_reports_the_retention_window(archive, keyring, active):
    write_records(active, start=1, count=8)
    rotate(archive)
    seal(archive, keyring)
    result = prune(archive, "1h")
    assert set(result) >= {"window", "removed", "dry_run"}
    assert result["window"] == "1h"
    assert result["dry_run"] is False
