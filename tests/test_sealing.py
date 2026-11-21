"""Tests for rotation, sealing and chain verification."""

from __future__ import annotations

import json

from rotseal import seal, seal_segment, verify_chain
from rotseal.rotate import rotate
from rotseal.util import (
    active_path,
    load_index,
    segment_filename,
    segments_dir,
)

from conftest import write_records


def test_write_rotate_seal_verify(archive, keyring, active):
    write_records(active, start=1, count=100)

    created = rotate(archive)
    assert len(created) == 1

    sealed = seal(archive, keyring)
    assert sealed == [created[0]["number"]]

    report = verify_chain(archive, keyring, from_segment=0)
    assert report.ok, report.failures
    assert report.exit_code == 0
    assert len(report.checked) == 1
    assert report.checked[0]["count"] == 100
    assert report.checked[0]["seq_start"] == 1
    assert report.checked[0]["seq_end"] == 100


def test_sidecar_records_the_block(archive, keyring, active):
    write_records(active, start=1, count=25)
    rotate(archive)
    seal(archive, keyring)

    path = segments_dir(archive) / "segment-000001.seal"
    doc = json.loads(path.read_text(encoding="utf-8"))
    for field in (
        "version",
        "segment",
        "seq_start",
        "seq_end",
        "count",
        "prev_tip",
        "records_digest",
        "sealed_length",
        "key_id",
        "alg",
        "sealed_at",
        "signature",
        "tip",
    ):
        assert field in doc, field
    assert doc["segment"] == 1
    assert doc["count"] == 25
    assert doc["seq_start"] == 1
    assert doc["seq_end"] == 25
    assert doc["prev_tip"] is None
    assert doc["key_id"] == "key-a"


def test_seal_segment_directly(archive, keyring, active):
    write_records(active, start=5, count=10)
    rotate(archive)
    doc = seal_segment(archive, 1, keyring)
    assert doc["count"] == 10
    assert doc["seq_start"] == 5
    assert doc["seq_end"] == 14
    assert len(doc["signature"]) == 64


def test_two_rotations_chain_together(archive, keyring, active):
    write_records(active, start=1, count=30)
    rotate(archive)
    write_records(active, start=31, count=30)
    rotate(archive)

    sealed = seal(archive, keyring)
    assert sealed == [1, 2]

    report = verify_chain(archive, keyring, from_segment=0)
    assert report.ok, report.failures
    segments = [item["segment"] for item in report.checked]
    assert segments == [1, 2]


def test_verify_from_second_segment(archive, keyring, active):
    write_records(active, start=1, count=20)
    rotate(archive)
    write_records(active, start=21, count=20)
    rotate(archive)
    seal(archive, keyring)

    report = verify_chain(archive, keyring, from_segment=2)
    assert report.ok, report.failures
    assert [item["segment"] for item in report.checked] == [2]


def test_verify_bounded_range(archive, keyring, active):
    for start in (1, 11, 21):
        write_records(active, start=start, count=10)
        rotate(archive)
    seal(archive, keyring)

    report = verify_chain(archive, keyring, from_segment=2, to_segment=3)
    assert report.ok, report.failures
    assert [item["segment"] for item in report.checked] == [2, 3]


def test_report_names_key_per_segment(archive, keyring, active):
    write_records(active, start=1, count=10)
    rotate(archive)
    seal(archive, keyring)

    report = verify_chain(archive, keyring, from_segment=0)
    assert report.ok
    assert report.key_usage() == {"key-a": 1}
    assert report.checked[0]["key_id"] == "key-a"


def test_rotate_by_time_creates_hourly_segments(archive, keyring, active):
    stamps = [
        "2026-01-01T09:10:00Z",
        "2026-01-01T09:40:00Z",
        "2026-01-01T10:05:00Z",
        "2026-01-01T11:20:00Z",
    ]
    with active.open("w", encoding="utf-8", newline="\n") as handle:
        for seq, stamp in enumerate(stamps, start=1):
            record = {"seq": seq, "ts": stamp, "level": "info", "msg": f"m{seq}"}
            handle.write(json.dumps(record) + "\n")

    created = rotate(archive, every="1h")
    assert len(created) >= 2
    seal(archive, keyring)

    report = verify_chain(archive, keyring, from_segment=0)
    assert report.ok, report.failures
    assert sum(item["count"] for item in report.checked) == 4


def test_rotate_by_size_keeps_segments_parseable(archive, keyring, active):
    # Equal-length records so that a byte budget that is a whole multiple of
    # one record also lands on a record boundary.
    line_length = None
    with active.open("w", encoding="utf-8", newline="\n") as handle:
        for seq in range(1, 11):
            record = {
                "seq": f"{seq:04d}",
                "ts": "2026-01-01T00:00:00Z",
                "level": "info",
                "msg": "fixed",
            }
            line = json.dumps(record) + "\n"
            line_length = len(line.encode("utf-8"))
            handle.write(line)

    created = rotate(archive, max_bytes=line_length * 5)
    assert len(created) == 2
    seal(archive, keyring)

    report = verify_chain(archive, keyring, from_segment=0)
    assert report.ok, report.failures
    assert sum(item["count"] for item in report.checked) == 10
    assert report.checked[0]["seq_start"] == 1
    assert report.checked[-1]["seq_end"] == 10


def test_rotate_removes_active_file(archive, keyring, active):
    write_records(active, start=1, count=5)
    rotate(archive)
    assert not active_path(archive).exists()

    index = load_index(archive)
    assert len(index["segments"]) == 1
    assert (segments_dir(archive) / segment_filename(1)).exists()


def test_rotate_empty_active_file(archive, active):
    active.write_text("", encoding="utf-8")
    created = rotate(archive)
    assert created == []
    assert not active.exists()


def test_verify_empty_archive(archive, keyring):
    report = verify_chain(archive, keyring, from_segment=0)
    assert report.ok
    assert report.checked == []
    assert report.exit_code == 0


def test_sealed_at_is_utc(archive, keyring, active):
    write_records(active, start=1, count=3)
    rotate(archive)
    doc = seal_segment(archive, 1, keyring)
    assert doc["sealed_at"].endswith("Z")


def test_rotate_size_chunks_are_bounded(archive, keyring, active):
    write_records(active, start=1, count=20)
    created = rotate(archive, max_bytes=500)
    assert len(created) > 1
    for entry in created:
        assert entry["bytes"] <= 500
    files = [
        (segments_dir(archive) / entry["file"]).stat().st_size for entry in created
    ]
    assert sum(entry["bytes"] for entry in created) == sum(files)
