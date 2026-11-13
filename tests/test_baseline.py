"""基线测试：覆盖当前实现里已经正确的行为。

新增测试请放在独立文件里，不要改这一份。
"""

from __future__ import annotations

import json

import pytest

from hlsmith.boxes import find_boxes, iter_boxes, read_u32
from hlsmith.init import build_init
from hlsmith.media import build_segment, read_tfdt_base_time
from hlsmith.packager import package
from hlsmith.payload import sample_payload
from hlsmith.playlist import build_playlist
from hlsmith.segmenter import Segment, segment
from hlsmith.source import (
    Sample,
    Source,
    format_source,
    make_source,
    parse_source,
    write_source,
)

TIMESCALE = 90000


def _source(seconds: float = 12.0, fps: int = 30, gop: int = 60) -> Source:
    return make_source(seconds=seconds, fps=fps, gop=gop)


# ---------------------------------------------------------------- source


def test_make_source_shape():
    source = _source(seconds=2.0, fps=30, gop=30)
    assert len(source.samples) == 60
    assert source.timescale == TIMESCALE
    assert source.duration_ticks == 2 * TIMESCALE
    assert source.samples[0].dts == 0
    assert [s.seq for s in source.samples] == list(range(60))


def test_source_roundtrip(tmp_path):
    source = _source(seconds=1.0)
    path = tmp_path / "src.jsonl"
    write_source(source, path)
    loaded = parse_source(path.read_text(encoding="utf-8").splitlines())
    assert loaded == source
    assert format_source(loaded) == path.read_text(encoding="utf-8")


def test_parse_rejects_missing_header():
    with pytest.raises(ValueError):
        parse_source(['{"seq": 0, "dts": 0, "dur": 3000, "sync": true, "size": 10}'])


def test_parse_rejects_non_monotonic_dts():
    lines = [
        json.dumps({"hlsmith_source": 1, "timescale": TIMESCALE}),
        json.dumps({"seq": 0, "dts": 0, "dur": 3000, "sync": True, "size": 10}),
        json.dumps({"seq": 1, "dts": 1000, "dur": 3000, "sync": False, "size": 10}),
    ]
    with pytest.raises(ValueError):
        parse_source(lines)


def test_parse_rejects_first_sample_not_sync():
    lines = [
        json.dumps({"hlsmith_source": 1, "timescale": TIMESCALE}),
        json.dumps({"seq": 0, "dts": 0, "dur": 3000, "sync": False, "size": 10}),
    ]
    with pytest.raises(ValueError):
        parse_source(lines)


# ---------------------------------------------------------------- payload


def test_payload_is_deterministic_and_sized():
    assert sample_payload(7, 100) == sample_payload(7, 100)
    assert len(sample_payload(7, 100)) == 100
    assert sample_payload(7, 100) != sample_payload(8, 100)
    assert sample_payload(7, 0) == b""


# ---------------------------------------------------------------- segmenter


def test_segments_cover_all_samples_without_overlap():
    source = _source(seconds=12.0)
    chunks = segment(source, 2.0)
    assert sum(len(chunk.samples) for chunk in chunks) == len(source.samples)
    for before, after in zip(chunks, chunks[1:]):
        assert before.last_seq + 1 == after.first_seq
    assert [chunk.index for chunk in chunks] == list(range(len(chunks)))


def test_first_segment_starts_at_source_start():
    source = _source(seconds=6.0)
    chunks = segment(source, 2.0)
    assert chunks[0].start_dts == source.samples[0].dts
    assert chunks[-1].end_dts == source.end_dts


def test_segment_duration_uses_sample_durations():
    samples = (
        Sample(seq=0, dts=0, duration=2000, sync=True, size=10),
        Sample(seq=1, dts=2000, duration=2500, sync=False, size=10),
        Sample(seq=2, dts=4500, duration=3000, sync=True, size=10),
    )
    source = Source(timescale=TIMESCALE, samples=samples)
    chunk = Segment(index=0, samples=samples[:2])
    assert chunk.duration_ticks == 4500
    assert chunk.duration_seconds(TIMESCALE) == pytest.approx(0.05)


# ---------------------------------------------------------------- init


def test_init_structure():
    data = build_init(TIMESCALE, 4 * TIMESCALE)
    top = [ref.kind for ref in iter_boxes(data)]
    assert top == ["ftyp", "moov"]
    assert find_boxes(data, ["moov", "mvhd"])
    assert find_boxes(data, ["moov", "trak", "mdia", "hdlr"])
    hdlr = find_boxes(data, ["moov", "trak", "mdia", "hdlr"])[0]
    assert data[hdlr.payload_start + 8 : hdlr.payload_start + 12] == b"vide"


def test_init_has_no_sample_tables():
    data = build_init(TIMESCALE, TIMESCALE)
    for name in ("stts", "stsc", "stsz", "stco"):
        ref = find_boxes(data, ["moov", "trak", "mdia", "minf", "stbl", name])[0]
        assert read_u32(data, ref.payload_start + 4) == 0


# ---------------------------------------------------------------- media


def test_segment_structure_and_payload():
    samples = (
        Sample(seq=3, dts=9000, duration=3000, sync=True, size=64),
        Sample(seq=4, dts=12000, duration=3000, sync=False, size=32),
    )
    data = build_segment(samples, timescale=TIMESCALE, base_decode_time=9000)
    top = [ref.kind for ref in iter_boxes(data)]
    assert top == ["moof", "mdat"]
    mdat = find_boxes(data, ["mdat"])[0]
    payload = data[mdat.payload_start : mdat.end]
    assert payload == sample_payload(3, 64) + sample_payload(4, 32)


def test_segment_mfhd_sequence_number():
    samples = (Sample(seq=0, dts=0, duration=3000, sync=True, size=8),)
    data = build_segment(samples, timescale=TIMESCALE, base_decode_time=0, sequence_number=7)
    mfhd = find_boxes(data, ["moof", "mfhd"])[0]
    assert read_u32(data, mfhd.payload_start + 4) == 7


def test_segment_tfdt_roundtrip():
    samples = (Sample(seq=0, dts=123456, duration=3000, sync=True, size=8),)
    data = build_segment(samples, timescale=TIMESCALE, base_decode_time=123456)
    assert read_tfdt_base_time(data) == 123456


# ---------------------------------------------------------------- playlist


def _segments(source: Source, target: float) -> list[Segment]:
    return segment(source, target)


def test_playlist_has_required_tags():
    source = _source(seconds=8.0)
    text = build_playlist(_segments(source, 2.0), TIMESCALE, target_seconds=2.0)
    lines = text.splitlines()
    assert lines[0] == "#EXTM3U"
    assert any(line.startswith("#EXT-X-VERSION:") for line in lines)
    assert any(line.startswith('#EXT-X-MAP:URI="init.mp4"') for line in lines)
    assert text.endswith("\n")


def test_playlist_extinf_count_matches_segments():
    source = _source(seconds=10.0)
    chunks = _segments(source, 2.0)
    text = build_playlist(chunks, TIMESCALE, target_seconds=2.0)
    assert text.count("#EXTINF:") == len(chunks)


def test_playlist_target_duration_is_integer():
    source = _source(seconds=10.0)
    text = build_playlist(_segments(source, 2.0), TIMESCALE, target_seconds=2.0)
    target_line = [l for l in text.splitlines() if l.startswith("#EXT-X-TARGETDURATION:")][0]
    assert int(target_line.split(":")[1]) >= 1


# ---------------------------------------------------------------- packager


def test_package_writes_expected_files(tmp_path):
    source = _source(seconds=10.0)
    result = package(source, tmp_path / "out", target_seconds=2.0)
    assert result.init_path.is_file()
    assert result.playlist_path.is_file()
    names = sorted(p.name for p in result.out_dir.iterdir())
    assert names == sorted(
        ["init.mp4", "index.m3u8"] + [p.name for p in result.segment_paths]
    )


def test_package_segment_files_are_moof_mdat(tmp_path):
    source = _source(seconds=6.0)
    result = package(source, tmp_path / "out", target_seconds=2.0)
    for path in result.segment_paths:
        kinds = [ref.kind for ref in iter_boxes(path.read_bytes())]
        assert kinds == ["moof", "mdat"]
